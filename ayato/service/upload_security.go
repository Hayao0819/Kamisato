package service

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/platform"
	pacmanpkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

func (v *uploadValidator) verifySignature(
	pkgFile, sigFile platform.SeekFile,
	storedName string,
) error {
	if sigFile == nil {
		if v.service.cfg != nil && v.service.cfg.RequireSign {
			return fmt.Errorf(
				"%w: package signature is required but none was provided",
				domain.ErrInvalidUpload,
			)
		}
		return nil
	}
	if sigFile.FileName() != storedName+".sig" {
		return fmt.Errorf(
			"%w: signature filename %q does not match package %q",
			domain.ErrInvalidUpload,
			sigFile.FileName(),
			storedName,
		)
	}
	if v.keyring == nil {
		return errors.New(
			"package signature present but no trust root is configured to validate it",
		)
	}
	if err := platform.Rewind(pkgFile); err != nil {
		return errors.WrapErr(err, "failed to seek package file for verification")
	}
	if err := platform.Rewind(sigFile); err != nil {
		return errors.WrapErr(err, "failed to seek signature file for verification")
	}
	fingerprint, err := v.keyring.VerifyDetached(pkgFile, sigFile)
	if err != nil {
		return fmt.Errorf(
			"%w: package signature verification failed: %s",
			domain.ErrInvalidUpload,
			err.Error(),
		)
	}
	slog.Info("package signature verified", "fingerprint", fingerprint)
	return nil
}

func (v *uploadValidator) checkBuildinfoProvenance(pkgFile platform.SeekFile) error {
	if v.service.cfg == nil || !v.service.cfg.RequireBuildinfoProvenance {
		return nil
	}
	if err := platform.Rewind(pkgFile); err != nil {
		return errors.WrapErr(err, "failed to seek package file for buildinfo check")
	}
	buildInfo, err := pacmanpkg.ReadBuildInfo(pkgFile)
	if err != nil {
		if errors.Is(err, pacmanpkg.ErrBuildInfoNotFound) {
			return fmt.Errorf(
				"%w: package has no .BUILDINFO but provenance is required",
				domain.ErrInvalidUpload,
			)
		}
		return fmt.Errorf(
			"%w: failed to read package .BUILDINFO: %s",
			domain.ErrInvalidUpload,
			err.Error(),
		)
	}
	expected := v.service.cfg.ExpectedBuildDir()
	if buildInfo.BuildDir != expected {
		return fmt.Errorf(
			"%w: package builddir %q is not the expected sandbox root %q",
			domain.ErrInvalidUpload,
			buildInfo.BuildDir,
			expected,
		)
	}
	return nil
}

func (v *uploadValidator) checkProtectedNames(info *raiou.PKGINFO) error {
	candidates := make([]string, 0, 1+len(info.Provides)+len(info.Replaces)+len(info.Group))
	candidates = append(candidates, info.PkgName)
	for _, provided := range info.Provides {
		candidates = append(candidates, dependencyName(provided))
	}
	for _, replaced := range info.Replaces {
		candidates = append(candidates, dependencyName(replaced))
	}
	candidates = append(candidates, info.Group...)
	for _, candidate := range candidates {
		if _, protected := v.protectedNames[candidate]; protected {
			return fmt.Errorf(
				"%w: package %q collides with protected official name %q",
				domain.ErrConflict,
				info.PkgName,
				candidate,
			)
		}
	}
	return nil
}

func dependencyName(entry string) string {
	if index := strings.IndexAny(entry, "=<>"); index >= 0 {
		return entry[:index]
	}
	return entry
}
