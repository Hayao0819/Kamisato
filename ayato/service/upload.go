package service

import (
	"fmt"
	"io"
	"log/slog"
	"path"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
)

// UploadFile uploads a package file and updates the DB and metadata.
// TODO: support signed DB, check gnupgDir existence
func (s *Service) UploadFile(repo string, files *domain.UploadFiles) error {
	slog.Info("upload pkg file", "file", files.PkgFile.FileName())
	if s.pkgBinaryRepo.VerifyPkgRepo(repo) != nil {
		slog.Warn("repository directory not found", "repo", repo)
		if err := s.pkgBinaryRepo.Init(repo, false, nil); err != nil {
			return fmt.Errorf("init repo err: %s", err.Error())
		}
	}
	pkgFileStream := files.PkgFile
	p, err := pkg.ReadBinaryPackage(pkgFileStream.FileName(), pkgFileStream)
	if err != nil {
		return fmt.Errorf("get pkg from bin err: %s", err.Error())
	}
	pi := p.PKGINFO()
	slog.Info("get pkg from bin", "pkgname", pi.PkgName, "pkgver", pi.PkgVer)

	// storeArch is where the file physically lives: an arch=any package is stored
	// once under "any/" and shared by every arch via FetchFile's fallback. dbArches
	// are the concrete architectures whose database registers the package.
	storeArch := pi.Arch
	dbArches, err := s.targetArches(repo, pi.Arch)
	if err != nil {
		return err
	}

	storedName := path.Base(pkgFileStream.FileName())
	useSignedDB := false // TODO: support signed DB
	var gnupgDir *string // TODO: check gnupgDir existence
	var added []string
	cleanup := func() {
		for _, arch := range added {
			if rmErr := s.pkgBinaryRepo.RepoRemove(repo, arch, pi.PkgName, useSignedDB, gnupgDir); rmErr != nil {
				slog.Warn("failed to roll back repo-add", "repo", repo, "arch", arch, "pkg", pi.PkgName, "err", rmErr)
			}
		}
		if delErr := s.pkgBinaryRepo.DeleteFile(repo, storeArch, storedName); delErr != nil {
			slog.Warn("failed to clean up stored pkg file after upload error", "repo", repo, "arch", storeArch, "filename", storedName, "err", delErr)
		}
	}

	if _, err := pkgFileStream.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek pkg file err: %s", err.Error())
	}
	if err := s.pkgBinaryRepo.StoreFile(repo, storeArch, pkgFileStream); err != nil {
		return fmt.Errorf("store file err: %s", err.Error())
	}

	for _, arch := range dbArches {
		if _, err := pkgFileStream.Seek(0, io.SeekStart); err != nil {
			cleanup()
			return fmt.Errorf("seek pkg file err: %s", err.Error())
		}
		if err := s.pkgBinaryRepo.RepoAdd(repo, arch, pkgFileStream, nil, useSignedDB, gnupgDir); err != nil {
			cleanup()
			return fmt.Errorf("repo-add err: %s", err.Error())
		}
		added = append(added, arch)
	}

	if err := s.pkgNameRepo.StorePackageFile(pi.PkgName, storedName); err != nil {
		cleanup()
		return fmt.Errorf("store pkg file name err: %s", err.Error())
	}
	return nil
}

// targetArches resolves the architecture databases an uploaded package is
// registered in. A concrete arch maps to itself. arch=any expands to every
// configured arch of the repo, because pacman has no os/any database: an any
// package must be in each arch's db to be installable there (its file is shared
// from the "any/" directory by FetchFile).
func (s *Service) targetArches(repo, pkgArch string) ([]string, error) {
	if pkgArch != "any" {
		return []string{pkgArch}, nil
	}
	rc := s.cfg.Repo(repo)
	if rc == nil {
		return nil, fmt.Errorf("repository %q is not configured; cannot place an arch=any package", repo)
	}
	arches := make([]string, 0, len(rc.Arches))
	for _, a := range rc.Arches {
		if a != "" && a != "any" {
			arches = append(arches, a)
		}
	}
	if len(arches) == 0 {
		return nil, fmt.Errorf("repository %q has no architectures configured for an arch=any package", repo)
	}
	return arches, nil
}
