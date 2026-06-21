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
	// Verify repository directory
	if s.pkgBinaryRepo.VerifyPkgRepo(repo) != nil {
		slog.Warn("repository directory not found", "repo", repo)
		if err := s.pkgBinaryRepo.Init(repo, false, nil); err != nil {
			return fmt.Errorf("init repo err: %s", err.Error())
		}
	}
	pkgFileStream := files.PkgFile
	// sigFileStream := files.SigFile
	// Get package file name
	p, err := pkg.ReadBinaryPackage(pkgFileStream.FileName(), pkgFileStream)
	if err != nil {
		return fmt.Errorf("get pkg from bin err: %s", err.Error())
	}
	if _, err := pkgFileStream.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek pkg file err: %s", err.Error())
	}
	// Get package info
	pi := p.PKGINFO()
	slog.Info("get pkg from bin", "pkgname", pi.PkgName, "pkgver", pi.PkgVer)
	// Store file
	if err := s.pkgBinaryRepo.StoreFile(repo, pi.Arch, pkgFileStream); err != nil {
		slog.Debug("store file success", "repo", repo, "arch", pi.Arch, "filename", pkgFileStream.FileName())
		return fmt.Errorf("store file err: %s", err.Error())
	}
	storedName := path.Base(pkgFileStream.FileName())
	// On later failure, best-effort remove the stored binary to avoid inconsistent state.
	cleanupStoredFile := func() {
		if delErr := s.pkgBinaryRepo.DeleteFile(repo, pi.Arch, storedName); delErr != nil {
			slog.Warn("failed to clean up stored pkg file after upload error", "repo", repo, "arch", pi.Arch, "filename", storedName, "err", delErr)
		}
	}
	// Update package DB
	useSignedDB := false // TODO: support signed DB
	var gnupgDir *string // TODO: check gnupgDir existence
	if err := s.pkgBinaryRepo.RepoAdd(repo, pi.Arch, pkgFileStream, nil, useSignedDB, gnupgDir); err != nil {
		cleanupStoredFile()
		return fmt.Errorf("repo-add err: %s", err.Error())
	}
	// Store metadata
	if err := s.pkgNameRepo.StorePackageFile(pi.PkgName, storedName); err != nil {
		cleanupStoredFile()
		return fmt.Errorf("store pkg file name err: %s", err.Error())
	}
	return nil
}
