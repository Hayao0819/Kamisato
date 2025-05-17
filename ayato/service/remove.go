package service

import (
	"fmt"
	"log/slog"
)

func (s *Service) RemovePkgFile(rname string, arch string, pkgname string) error {
	// Verify repository directory
	if s.r.VerifyPkgRepo(rname) != nil {
		slog.Warn("repository directory not found", "repo", rname)
		if err := s.r.Init(arch, false, nil); err != nil {
			return fmt.Errorf("init repo err: %s", err.Error())
		}
	}

	// Package file
	filename, err := s.r.GetPkgFileName(pkgname)
	if err != nil {
		return fmt.Errorf("get pkg file err: %s", err.Error())
	}
	slog.Info("get pkg file", "file", filename)

	if err := s.r.DeleteFile(rname, arch, filename, false, nil); err != nil {
		return fmt.Errorf("delete pkg file err: %s", err.Error())
	}

	if err := s.r.DeletePkgFileName(pkgname); err != nil {
		return fmt.Errorf("remove pkg file name err: %s", err.Error())
	}

	return nil
}
