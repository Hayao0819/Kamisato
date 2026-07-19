package service

import (
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/platform"
	"github.com/Hayao0819/Kamisato/internal/limits"
	pacmanpkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

type uploadValidator struct {
	service        *Service
	keyring        *sign.Keyring
	protectedNames map[string]struct{}
}

func (p *uploadPublication) validateInputs() error {
	keyring, err := p.batchKeyring()
	if err != nil {
		return err
	}
	validator := newUploadValidator(p.service, keyring)
	p.uploads = make([]preparedUpload, 0, len(p.files))
	seenObjects := make(map[archKey]struct{}, len(p.files))
	for _, files := range p.files {
		upload, err := validator.validate(files)
		if err != nil {
			return err
		}
		key := archKey{arch: upload.storeArch, key: upload.storedName}
		if _, duplicate := seenObjects[key]; duplicate {
			return fmt.Errorf(
				"%w: duplicate package object %q in one batch",
				domain.ErrInvalidUpload,
				upload.storedName,
			)
		}
		seenObjects[key] = struct{}{}
		p.uploads = append(p.uploads, upload)
	}
	return nil
}

func (p *uploadPublication) batchKeyring() (*sign.Keyring, error) {
	for _, files := range p.files {
		if files != nil && files.SigFile != nil {
			keyring, err := p.service.verifyKeyring()
			if err != nil {
				return nil, errors.WrapErr(err, "build signature keyring")
			}
			return keyring, nil
		}
	}
	return nil, nil
}

func newUploadValidator(service *Service, keyring *sign.Keyring) *uploadValidator {
	validator := &uploadValidator{service: service, keyring: keyring}
	if service.cfg != nil {
		validator.protectedNames = make(
			map[string]struct{},
			len(service.cfg.ProtectedNames),
		)
		for _, name := range service.cfg.ProtectedNames {
			validator.protectedNames[name] = struct{}{}
		}
	}
	return validator
}

func (v *uploadValidator) validate(files *domain.UploadFiles) (preparedUpload, error) {
	if files == nil || files.PkgFile == nil {
		return preparedUpload{}, fmt.Errorf("%w: package file is required", domain.ErrInvalidUpload)
	}
	pkgStream := files.PkgFile
	if err := v.checkSize(pkgStream); err != nil {
		return preparedUpload{}, err
	}
	binary, err := pacmanpkg.ReadBinaryPackage(pkgStream.FileName(), pkgStream)
	if err != nil {
		return preparedUpload{}, fmt.Errorf(
			"%w: failed to read package from binary: %w",
			domain.ErrInvalidUpload,
			err,
		)
	}
	info := binary.PKGINFO()
	slog.Info("read uploaded package", "pkgname", info.PkgName, "pkgver", info.PkgVer)
	storedName, err := validatedPackageFilename(pkgStream.FileName(), info)
	if err != nil {
		return preparedUpload{}, err
	}
	if err := v.checkBuildinfoProvenance(pkgStream); err != nil {
		return preparedUpload{}, err
	}
	if err := v.checkProtectedNames(info); err != nil {
		return preparedUpload{}, err
	}
	if err := v.verifySignature(pkgStream, files.SigFile, storedName); err != nil {
		return preparedUpload{}, err
	}
	if err := validatePackageArch(info.Arch); err != nil {
		return preparedUpload{}, err
	}

	upload := preparedUpload{
		pkgStream:  pkgStream,
		pkgName:    info.PkgName,
		pkgVersion: info.PkgVer,
		storeArch:  info.Arch,
		storedName: storedName,
		sigName:    storedName + ".sig",
	}
	if files.SigFile != nil {
		upload.sigStream = files.SigFile
	}
	return upload, nil
}

func (v *uploadValidator) checkSize(file platform.SeekFile) error {
	current, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return errors.WrapErr(err, "failed to inspect package size")
	}
	size, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return errors.WrapErr(err, "failed to inspect package size")
	}
	if _, err := file.Seek(current, io.SeekStart); err != nil {
		return errors.WrapErr(err, "failed to restore package stream")
	}
	maxSize := 0
	if v.service.cfg != nil {
		maxSize = v.service.cfg.MaxSize
	}
	if limits.Exceeds(size, maxSize) {
		return fmt.Errorf(
			"%w: package exceeds max_size (%d > %d bytes)",
			domain.ErrInvalidUpload,
			size,
			limits.PackageBytes(maxSize),
		)
	}
	return nil
}

func validatedPackageFilename(fileName string, info *raiou.PKGINFO) (string, error) {
	artifact, err := pacmanpkg.ParseArtifact(fileName)
	if err != nil || artifact.IsSignature() {
		return "", fmt.Errorf("%w: invalid package filename %q", domain.ErrInvalidUpload, fileName)
	}
	coordinates, err := artifact.Coordinates()
	if err != nil || !coordinates.MatchesMetadata(info.PkgName, info.PkgVer, info.Arch) {
		return "", fmt.Errorf(
			"%w: package filename %q does not match .PKGINFO (%s, %s, %s)",
			domain.ErrInvalidUpload,
			fileName,
			info.PkgName,
			info.PkgVer,
			info.Arch,
		)
	}
	return artifact.Filename(), nil
}

func validatePackageArch(arch string) error {
	if arch == "" || strings.ContainsRune(arch, '/') || strings.Contains(arch, "..") {
		return fmt.Errorf("%w: invalid package arch %q", domain.ErrInvalidUpload, arch)
	}
	return nil
}
