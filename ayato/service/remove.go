package service

import (
	"fmt"
	"log/slog"
)

func (s *Service) RemovePkgFile(rname string, arch string, pkgname string) error {
	// Verify repository directory
	if s.r.VerifyPkgRepo(rname) != nil {
		slog.Warn("repository directory not found", "repo", rname)
		if err := s.r.Init(false, nil); err != nil {
			return fmt.Errorf("init repo err: %s", err.Error())
		}
	}

	// Package file
	filename, err := s.r.GetPkgFileName(pkgname)
	if err != nil {
		return fmt.Errorf("get pkg file err: %s", err.Error())
	}

	// Remove package file to the repository directory
	// pkgPath := path.Join(repoDir, "x86_64", filename)
	// slog.Info("remove pkg file", "file", pkgPath)
	// if err := os.Remove(pkgPath); err != nil {
	// 	slog.Warn("remove pkg file err", "err", err)
	// }

	if err := s.r.DeleteFile(rname, "x86_64", filename, false, nil); err != nil {
		return fmt.Errorf("delete pkg file err: %s", err.Error())
	}

	return nil
}
