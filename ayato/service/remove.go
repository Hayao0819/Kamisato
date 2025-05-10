package service

import (
	"fmt"
	"log/slog"
	"os"
	"path"
)

func (s *Service) RemovePkgFile(rname string, pkgname string) error {
	// Correct information
	repoDir, err := s.repo.PkgRepoDir(rname)
	if err != nil {
		// return fmt.Errorf("repo %s not found", repo)
		return err
	}

	// Package file
	filename, err := s.repo.GetPkgFileName(pkgname)
	if err != nil {
		return fmt.Errorf("get pkg file err: %s", err.Error())
	}

	// Remove package file to the repository directory
	pkgPath := path.Join(repoDir, "x86_64", filename)
	slog.Info("remove pkg file", "file", pkgPath)
	if err := os.Remove(pkgPath); err != nil {
		slog.Warn("remove pkg file err", "err", err)
	}

	// Update the package kv database
	if err := s.repo.DeletePkgFileName(rname); err != nil {
		return fmt.Errorf("delete pkg file err: %s", err.Error())
	}

	// Remove package from the database
	if err := s.repo.PkgRepoRemove(rname, pkgname, false, nil); err != nil {
		return fmt.Errorf("repo-remove err: %s", err.Error())
	}

	return nil
}
