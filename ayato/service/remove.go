package service

import (
	"fmt"
	"log/slog"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

// RemovePkg はパッケージファイルとメタ情報を削除します。
func (s *Service) RemovePkg(rname string, arch string, pkgname string) error {
	if err := s.ValidateRepoName(rname); err != nil {
		slog.Error("validate repo name failed", "repo", rname, "error", err.Error())
		return utils.WrapErr(err, "validate repo name failed")
	}
	// パッケージファイル名取得
	filename, err := s.pkgNameRepo.GetPkgFileName(pkgname)
	if err != nil {
		return fmt.Errorf("get pkg file err: %s", err.Error())
	}
	slog.Info("get pkg file", "file", filename)
	// ファイル削除
	if err := s.pkgBinaryRepo.DeleteFile(rname, arch, filename); err != nil {
		slog.Debug("delete file success", "repo", rname, "arch", arch, "filename", filename)
		return fmt.Errorf("delete pkg file err: %s", err.Error())
	}
	// メタ情報削除
	if err := s.pkgNameRepo.DeletePkgFileName(pkgname); err != nil {
		slog.Debug("delete pkg file name success", "pkgname", pkgname)
		return fmt.Errorf("remove pkg file name err: %s", err.Error())
	}
	return nil
}
