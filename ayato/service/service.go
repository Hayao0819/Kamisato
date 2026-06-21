package service

import (
	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

// Service はビジネスロジックを提供する構造体です。
type Service struct {
	pkgNameRepo   repository.NameStore        // パッケージ名管理
	pkgBinaryRepo repository.BinaryRepository // パッケージバイナリ管理
	cfg           *conf.AyatoConfig           // 設定
}

// Servicer は Service が提供する操作の全体インターフェースです（モック境界）。
//
//go:generate mockgen -source=service.go -destination=../test/mocks/service.go -package=mocks
type Servicer interface {
	InitAll() error

	// --- ファイル操作 ---
	GetFile(repoName, archName, name string) (stream.File, error)
	UploadFile(repo string, files *domain.UploadFiles) error
	RemovePkg(rname string, arch string, pkgname string) error
	SignedURL(repo string, arch string, name string) (string, error)

	// --- リポジトリ操作 ---
	RepoNames() ([]string, error)
	Arches(repo string) ([]string, error)
	Repo(repo string) (*domain.PacmanRepo, error)
	Pkgs(repo, arch string) (*domain.PacmanPkgs, error)
	PkgDetail(repo, arch, pkg string) (*raiou.PKGINFO, error)
	PkgFiles(repo, arch, pkg string) ([]string, error)
	RepoFileList(repo, arch string) ([]string, error)
}

// New はServiceのコンストラクタです。
func New(pkgNameRepo repository.NameStore, pkgBinaryRepo repository.BinaryRepository, config *conf.AyatoConfig) Servicer {
	return &Service{
		pkgNameRepo:   pkgNameRepo,
		pkgBinaryRepo: pkgBinaryRepo,
		cfg:           config,
	}
}
