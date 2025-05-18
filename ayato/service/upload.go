package service

import (
	"fmt"
	"log/slog"
	"path"

	repo "github.com/Hayao0819/Kamisato/alpm"
)

func (s *Service) UploadPkgFile(rname string, name [2]string) error {
	pkgFile := name[0]
	// sigFile := name[1]
	slog.Info("upload pkg file", "file", pkgFile)

	// Verify repository directory
	if s.r.VerifyPkgRepo(rname) != nil {
		slog.Warn("repository directory not found", "repo", rname)
		if err := s.r.Init(rname, false, nil); err != nil {
			return fmt.Errorf("init repo err: %s", err.Error())
		}
	}

	// Get package file name
	p, err := repo.GetPkgFromBin(pkgFile)
	if err != nil {
		return fmt.Errorf("get pkg from bin err: %s", err.Error())
	}

	// Get package information
	pi, err := p.PKGINFO()
	if err != nil {
		return fmt.Errorf("get pkginfo err: %s", err.Error())
	}
	slog.Info("get pkg from bin", "pkgname", pi.PkgName, "pkgver", pi.PkgVer)

	// Store package file to the repository directory
	useSignedDB := false
	var gnupgDir *string // TODO: Check if the directory exists
	if err := s.r.StoreFile(rname, pi.Arch, pkgFile, useSignedDB, gnupgDir); err != nil {
		return fmt.Errorf("store file err: %s", err.Error())
	}

	// Store metadata to the kv store
	if err := s.r.StorePkgFileName(pi.PkgName, path.Base(pkgFile)); err != nil {
		return fmt.Errorf("store pkg file name err: %s", err.Error())
	}

	return nil
}
