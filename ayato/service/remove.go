package service

import (
	"fmt"
	"log/slog"

	"github.com/cockroachdb/errors"
)

func (s *Service) RemovePkgFile(rname string, arch string, pkgname string) error {

	if err := s.ValidateRepoName(rname); err != nil {
		slog.Error("validate repo name failed", "repo", rname, "error", err.Error())
		return errors.Wrap(err, "validate repo name failed")
	}

	// Package file
	filename, err := s.r.GetPkgFileName(pkgname)
	if err != nil {
		return fmt.Errorf("get pkg file err: %s", err.Error())
	}
	slog.Info("get pkg file", "file", filename)

	if err := s.r.DeleteFile(rname, arch, filename); err != nil {
		slog.Debug("delete file success", "repo", rname, "arch", arch, "filename", filename)
		return fmt.Errorf("delete pkg file err: %s", err.Error())
	}

	if err := s.r.DeletePkgFileName(pkgname); err != nil {
		slog.Debug("delete pkg file name success", "pkgname", pkgname)
		return fmt.Errorf("remove pkg file name err: %s", err.Error())
	}

	return nil
}
