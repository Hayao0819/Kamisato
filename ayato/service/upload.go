package service

import (
	"fmt"
	"io"
	"log/slog"
	"path"

	"github.com/Hayao0819/Kamisato/alpm/pkg"
	"github.com/Hayao0819/Kamisato/ayato/domain"
)

func (s *Service) UploadPkgFile(repo string, files *domain.UploadFiles) error {
	slog.Info("upload pkg file", "file", files.PkgFile.FileName())

	// Verify repository directory
	if s.r.VerifyPkgRepo(repo) != nil {
		slog.Warn("repository directory not found", "repo", repo)
		if err := s.r.Init(repo, false, nil); err != nil {
			return fmt.Errorf("init repo err: %s", err.Error())
		}
	}

	pkgFileStream := files.PkgFile
	// sigFileStream := files.SigFile

	// Get package file name
	p, err := pkg.GetPkgFromBin(pkgFileStream.FileName(), pkgFileStream)
	if err != nil {
		return fmt.Errorf("get pkg from bin err: %s", err.Error())
	}
	pkgFileStream.Seek(0, io.SeekStart)

	// Get package information
	pi, err := p.PKGINFO()
	if err != nil {
		return fmt.Errorf("get pkginfo err: %s", err.Error())
	}
	slog.Info("get pkg from bin", "pkgname", pi.PkgName, "pkgver", pi.PkgVer)

	// Store package file to the repository directory
	if err := s.r.StoreFile(repo, pi.Arch, pkgFileStream); err != nil {
		return fmt.Errorf("store file err: %s", err.Error())
	}

	// Update the package database
	// TOOD: Support signed database
	useSignedDB := false
	var gnupgDir *string // TODO: Check if the directory exists
	if err := s.r.RepoAdd(repo, pi.Arch, pkgFileStream, nil, useSignedDB, gnupgDir); err != nil {
		return fmt.Errorf("repo-add err: %s", err.Error())
	}

	// Store metadata to the kv store
	if err := s.r.StorePkgFileName(pi.PkgName, path.Base(pkgFileStream.FileName())); err != nil {
		return fmt.Errorf("store pkg file name err: %s", err.Error())
	}

	return nil
}
