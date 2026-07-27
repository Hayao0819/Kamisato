package service

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

// maxSignatureBytes bounds a stored .sig read; a detached PGP signature is a
// few hundred bytes, so anything larger is corrupt or hostile.
const maxSignatureBytes = 64 << 10

// PkgSignature reports whether pkgname's stored signature exists and what its
// signature packet claims, without verifying it.
func (s *Service) PkgSignature(repo, arch, pkgname string) (*domain.PackageSignature, error) {
	filename, err := s.pkgFilename(repo, arch, pkgname)
	if err != nil {
		return nil, err
	}
	sigName := filename + ".sig"
	data, err := s.readRepoFile(repo, arch, sigName, maxSignatureBytes)
	if errors.Is(err, blob.ErrNotFound) {
		return &domain.PackageSignature{Present: false}, nil
	}
	if err != nil {
		return nil, errors.WrapErr(err, "read package signature")
	}
	result := &domain.PackageSignature{Present: true, Filename: sigName}
	info, err := sign.InspectDetached(data)
	if err != nil {
		slog.Warn("failed to parse stored package signature", "repo", repo, "arch", arch, "file", sigName, "err", err)
		return result, nil
	}
	result.KeyID = info.KeyID
	result.Fingerprint = info.Fingerprint
	result.CreatedAt = info.CreatedAt.Unix()
	result.Hash = info.Hash
	result.PubKeyAlgo = info.PubKeyAlgo
	return result, nil
}

func (s *Service) pkgFilename(repo, arch, pkgname string) (string, error) {
	rr, err := s.pkgBinaryRepo.RemoteRepo(repo, arch)
	if err != nil {
		return "", classifyRepositoryRead(err)
	}
	pkg := rr.PkgByPkgName(pkgname)
	if pkg == nil {
		return "", fmt.Errorf(
			"%w: package %q not found in repository %s/%s",
			domain.ErrNotFound,
			pkgname,
			repo,
			arch,
		)
	}
	return pkg.Path(), nil
}

func (s *Service) readRepoFile(repo, arch, name string, limit int64) ([]byte, error) {
	f, err := s.pkgBinaryRepo.FetchFile(repo, arch, name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s exceeds %d bytes", name, limit)
	}
	return data, nil
}
