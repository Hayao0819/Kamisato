package service

import (
	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

type Service struct {
	r   repository.IRepository
	cfg *conf.AyatoConfig
}

//go:generate mockgen -source=service.go -destination=../test/mocks/service.go -package=mocks
type IService interface {
	InitAll() error

	IFileService
	IPacmanRepoService
}

type IFileService interface {
	// ファイルを取得します
	GetFile(repoName, archName, name string) (stream.IFileStream, error)
	// ファイルをアップロードします
	UploadFile(repo string, files *domain.UploadFiles) error
	// パッケージファイルを削除します
	RemovePkg(rname string, arch string, pkgname string) error
}

type IPacmanRepoService interface {
	// 全てのリポジトリ名を返します
	RepoNames() ([]string, error)
	// 特定のリポジトリのアーキテクチャの一覧を返します
	Arches(repo string) ([]string, error)
	// アーキテクチャのリスト、パッケージのリスト等のパッケージリポジトリに関する全ての情報を返します
	Repo(repo string) (*domain.PacmanRepo, error)
	// 全てのパッケージのメタ情報を返します
	Pkgs(repo, arch string) (*domain.PacmanPkgs, error)
	// 特定のパッケージの詳細情報を返します
	PkgDetail(repo, arch, pkg string) (*raiou.PKGINFO, error)
	// 特定のパッケージのファイル一覧を返します
	PkgFiles(repo, arch, pkg string) ([]string, error)
	// 特定のリポジトリのアーキテクチャのファイル一覧を返します
	RepoFileList(repo, arch string) ([]string, error)
}

func New(repo repository.IRepository, config *conf.AyatoConfig) IService {
	return &Service{
		r:   repo,
		cfg: config,
	}
}
