package service

import (
	"fmt"
	"log/slog"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/pkg/pacman/gpg"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

// Service provides the business logic.
type Service struct {
	pkgNameRepo   repository.NameStore
	pkgBinaryRepo repository.BinaryRepository
	cfg           *conf.AyatoConfig

	// verifier is the loaded package-signature trust root, nil when no keyring
	// is configured. verifierErr records a fail-closed startup error (a load
	// failure, or RequireSign without a usable trust root) surfaced by InitAll.
	verifier    *gpg.Keyring
	verifierErr error
}

// Servicer is the full interface of operations Service provides (mock boundary).
//
//go:generate mockgen -source=service.go -destination=../test/mocks/service.go -package=mocks
type Servicer interface {
	InitAll() error

	GetFile(repoName, archName, name string) (stream.File, error)
	UploadFile(repo string, files *domain.UploadFiles) error
	RemovePkg(rname string, arch string, pkgname string) error
	SignedURL(repo string, arch string, name string) (string, error)

	RepoNames() ([]string, error)
	Arches(repo string) ([]string, error)
	Repo(repo string) (*domain.PacmanRepo, error)
	Pkgs(repo, arch string) (*domain.PacmanPkgs, error)
	PkgDetail(repo, arch, pkg string) (*raiou.PKGINFO, error)
	PkgFiles(repo, arch, pkg string) ([]string, error)
	RepoFileList(repo, arch string) ([]string, error)
}

func New(pkgNameRepo repository.NameStore, pkgBinaryRepo repository.BinaryRepository, config *conf.AyatoConfig) Servicer {
	s := &Service{
		pkgNameRepo:   pkgNameRepo,
		pkgBinaryRepo: pkgBinaryRepo,
		cfg:           config,
	}

	// Load the trust root for package-signature verification. The keyring is a
	// dedicated public-key file, separate from Build.GnupgHome (the signing
	// private key). When RequireSign is set we cannot fail closed without a
	// trust root, so a missing or unloadable keyring is a startup error
	// surfaced by InitAll; otherwise the verifier simply stays nil.
	if config != nil && config.Verify.Keyring != "" {
		kr, err := gpg.LoadKeyring(config.Verify.Keyring, config.Verify.TrustedKeys)
		if err != nil {
			s.verifierErr = fmt.Errorf("load package-signature keyring: %w", err)
			slog.Error("failed to load package-signature keyring", "path", config.Verify.Keyring, "err", err)
		} else {
			s.verifier = kr
			slog.Info("package-signature verification enabled", "keyring", config.Verify.Keyring, "trusted_keys", len(config.Verify.TrustedKeys))
		}
	}
	if config != nil && config.RequireSign && s.verifier == nil && s.verifierErr == nil {
		s.verifierErr = fmt.Errorf("require_sign is enabled but no verify.keyring is configured; cannot fail closed without a trust root")
	}

	return s
}
