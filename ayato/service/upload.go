package service

import (
	"fmt"
	"io"
	"log/slog"
	"path"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/package"
)

// UploadFile はパッケージファイルをアップロードし、DB・メタ情報を更新します。
// TODO: 署名付きDB対応、gnupgDir存在チェック
func (s *Service) UploadFile(repo string, files *domain.UploadFiles) error {
	slog.Info("upload pkg file", "file", files.PkgFile.FileName())
	// リポジトリディレクトリ検証
	if s.r.VerifyPkgRepo(repo) != nil {
		slog.Warn("repository directory not found", "repo", repo)
		if err := s.r.Init(repo, false, nil); err != nil {
			return fmt.Errorf("init repo err: %s", err.Error())
		}
	}
	pkgFileStream := files.PkgFile
	// sigFileStream := files.SigFile
	// パッケージファイル名取得
	p, err := pkg.GetPkgFromBin(pkgFileStream.FileName(), pkgFileStream)
	if err != nil {
		return fmt.Errorf("get pkg from bin err: %s", err.Error())
	}
	if _, err := pkgFileStream.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek pkg file err: %s", err.Error())
	}
	// パッケージ情報取得
	pi, err := p.PKGINFO()
	if err != nil {
		return fmt.Errorf("get pkginfo err: %s", err.Error())
	}
	slog.Info("get pkg from bin", "pkgname", pi.PkgName, "pkgver", pi.PkgVer)
	// ファイル保存
	if err := s.r.StoreFile(repo, pi.Arch, pkgFileStream); err != nil {
		slog.Debug("store file success", "repo", repo, "arch", pi.Arch, "filename", pkgFileStream.FileName())
		return fmt.Errorf("store file err: %s", err.Error())
	}
	// パッケージDB更新
	useSignedDB := false // TODO: 署名付きDB対応
	var gnupgDir *string // TODO: gnupgDir存在チェック
	if err := s.r.RepoAdd(repo, pi.Arch, pkgFileStream, nil, useSignedDB, gnupgDir); err != nil {
		return fmt.Errorf("repo-add err: %s", err.Error())
	}
	// メタ情報保存
	if err := s.r.StorePkgFileName(pi.PkgName, path.Base(pkgFileStream.FileName())); err != nil {
		slog.Debug("store pkg file name success", "pkgname", pi.PkgName, "filename", path.Base(pkgFileStream.FileName()))
		return fmt.Errorf("store pkg file name err: %s", err.Error())
	}
	return nil
}
