package service

import (
	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

// Service provides the business logic.
type Service struct {
	pkgNameRepo   repository.NameStore        // package name management
	pkgBinaryRepo repository.BinaryRepository // package binary management
	cfg           *conf.AyatoConfig           // config
}

// Servicer is the full interface of operations Service provides (mock boundary).
//
//go:generate mockgen -source=service.go -destination=../test/mocks/service.go -package=mocks
type Servicer interface {
	InitAll() error

	// --- file operations ---
	GetFile(repoName, archName, name string) (stream.File, error)
	UploadFile(repo string, files *domain.UploadFiles) error
	RemovePkg(rname string, arch string, pkgname string) error
	SignedURL(repo string, arch string, name string) (string, error)

	// --- repository operations ---
	RepoNames() ([]string, error)
	Arches(repo string) ([]string, error)
	Repo(repo string) (*domain.PacmanRepo, error)
	Pkgs(repo, arch string) (*domain.PacmanPkgs, error)
	PkgDetail(repo, arch, pkg string) (*raiou.PKGINFO, error)
	PkgFiles(repo, arch, pkg string) ([]string, error)
	RepoFileList(repo, arch string) ([]string, error)
}

// New is the constructor for Service.
func New(pkgNameRepo repository.NameStore, pkgBinaryRepo repository.BinaryRepository, config *conf.AyatoConfig) Servicer {
	return &Service{
		pkgNameRepo:   pkgNameRepo,
		pkgBinaryRepo: pkgBinaryRepo,
		cfg:           config,
	}
}
