package repository

import (
	"fmt"
	"path"

	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/samber/lo"
)

//go:generate mockgen -source=repository.go -destination=../test/mocks/repository.go -package=mocks

// Store はバイナリ（パッケージファイル・DB）を保存する低レベルなバックエンド契約です。
// localfs / s3 が直接実装します。
type Store interface {
	StoreFile(repo, arch string, file stream.SeekFile) error
	StoreFileWithSignedURL(repo, arch, name string) (string, error)
	DeleteFile(repo, arch, file string) error
	FetchFile(repo, arch, file string) (stream.File, error)
	RepoAdd(name, arch string, pkg, sig stream.SeekFile, useSignedDB bool, gnupgDir *string) error
	RepoRemove(name, arch, pkg string, useSignedDB bool, gnupgDir *string) error
	InitArch(name, arch string, useSignedDB bool, gnupgDir *string) error
	RepoNames() ([]string, error)
	Files(repo, arch string) ([]string, error)
	Arches(repo string) ([]string, error)
}

// NameStore はパッケージ名と保存ファイル名の対応を管理します（blinky 互換）。
type NameStore interface {
	PackageFile(name string) (string, error)
	StorePackageFile(packageName, filePath string) error
	DeletePackageFileEntry(packageName string) error
}

// BinaryRepository はサービス層が依存する高レベルなリポジトリです。
// 低レベルな Store に pacman 固有の派生操作を加えたものです。
type BinaryRepository interface {
	Store
	FetchDB(repoName, archName string) (stream.File, error)
	PkgNames(repoName, archName string) ([]string, error)
	RemoteRepo(name, arch string) (*repo.RemoteRepo, error)
	PkgFiles(repoName, archName, pkgName string) ([]string, error)
	// Init はリポジトリの全アーキテクチャ（設定 + 既存）を初期化します。
	Init(name string, useSignedDB bool, gnupgDir *string) error
	VerifyPkgRepo(name string) error
}

// binaryRepository は Store を埋め込み、派生操作だけを上乗せします
// （低レベルメソッドは埋め込みにより自動委譲され、ボイラープレートはありません）。
type binaryRepository struct {
	Store
	cfg *conf.AyatoConfig
}

// NewBinaryRepository は低レベル Store を派生操作付きの BinaryRepository に包みます。
func NewBinaryRepository(store Store, cfg *conf.AyatoConfig) BinaryRepository {
	return &binaryRepository{Store: store, cfg: cfg}
}

// FetchDB はリポジトリ・アーキテクチャの DB ファイルを取得します。
func (r *binaryRepository) FetchDB(repoName, archName string) (stream.File, error) {
	return r.FetchFile(repoName, archName, repoName+".db")
}

// RemoteRepo は DB を解析して RemoteRepo を返します。
func (r *binaryRepository) RemoteRepo(name, arch string) (*repo.RemoteRepo, error) {
	db, err := r.FetchDB(name, arch)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rr, err := repo.RemoteRepoFromDB(name, db)
	if err != nil {
		return nil, err
	}
	if rr == nil {
		return nil, fmt.Errorf("failed to get repository")
	}
	return rr, nil
}

// PkgNames はリポジトリ内の全パッケージの pkgbase を返します。
// FIXME: 毎回 DB を開くのは非効率。キャッシュ等での最適化が望ましい。
func (r *binaryRepository) PkgNames(repoName, archName string) ([]string, error) {
	db, err := r.FetchFile(repoName, archName, fmt.Sprintf("%s.db.tar.gz", repoName))
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rr, err := repo.RemoteRepoFromDB(repoName, db)
	if err != nil {
		return nil, err
	}
	if rr == nil {
		return nil, fmt.Errorf("failed to get repository")
	}
	names := make([]string, 0, len(rr.Pkgs))
	for _, p := range rr.Pkgs {
		names = append(names, p.Base())
	}
	return names, nil
}

// PkgFiles はリポジトリ内パッケージのファイル一覧を返します。
// TODO: 未実装（パッケージファイル一覧の取得）。
func (r *binaryRepository) PkgFiles(repoName, archName, pkgName string) ([]string, error) {
	db, err := r.FetchDB(repoName, archName)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if _, err := repo.RemoteRepoFromDB(repoName, db); err != nil {
		return nil, err
	}
	return nil, nil
}

// Init はリポジトリの全アーキテクチャ（設定 + 既存）を初期化します。
func (r *binaryRepository) Init(name string, useSignedDB bool, gnupgDir *string) error {
	createdArches, err := r.Arches(name)
	if err != nil {
		createdArches = []string{}
	}

	repoconfig := r.cfg.Repo(name)
	if repoconfig == nil {
		return fmt.Errorf("repository %s not found in config", name)
	}

	arches := lo.Uniq(append(append([]string{}, createdArches...), repoconfig.Arches...))
	for _, arch := range arches {
		if err := r.InitArch(name, arch, useSignedDB, gnupgDir); err != nil {
			return err
		}
	}
	return nil
}

// VerifyPkgRepo は各アーキテクチャに必須ファイルが揃っているか検証します。
func (r *binaryRepository) VerifyPkgRepo(name string) error {
	arches, err := r.Arches(name)
	if err != nil {
		return utils.WrapErr(err, "failed to get arches")
	}

	for _, arch := range arches {
		files, err := r.Files(name, arch)
		if err != nil {
			return utils.WrapErr(err, fmt.Sprintf("failed to get files for arch %s", arch))
		}

		requiredFiles := []string{
			name + ".db",
			name + ".db.tar.gz",
			name + ".files",
			name + ".files.tar.gz",
		}

		for _, file := range requiredFiles {
			if !lo.Contains(files, file) {
				return fmt.Errorf("%s not found in %s", file, path.Join(name, arch))
			}
		}
	}
	return nil
}
