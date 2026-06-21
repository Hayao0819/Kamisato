package service

import (
	"fmt"
	"log/slog"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

// RemovePkg deletes the package file and its metadata.
func (s *Service) RemovePkg(rname string, arch string, pkgname string) error {
	if err := s.ValidateRepoName(rname); err != nil {
		slog.Error("validate repo name failed", "repo", rname, "error", err.Error())
		return utils.WrapErr(err, "validate repo name failed")
	}
	// Get package file name (fall back to .db resolution on NameStore miss)
	filename, err := s.resolvePackageFile(rname, arch, pkgname)
	if err != nil {
		return fmt.Errorf("get pkg file err: %s", err.Error())
	}
	slog.Info("get pkg file", "file", filename)
	if err := s.pkgBinaryRepo.DeleteFile(rname, arch, filename); err != nil {
		slog.Debug("delete file success", "repo", rname, "arch", arch, "filename", filename)
		return fmt.Errorf("delete pkg file err: %s", err.Error())
	}
	if err := s.pkgNameRepo.DeletePackageFileEntry(pkgname); err != nil {
		slog.Debug("delete pkg file name success", "pkgname", pkgname)
		return fmt.Errorf("remove pkg file name err: %s", err.Error())
	}
	return nil
}
