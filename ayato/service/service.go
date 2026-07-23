package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/platform"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/pkg/httpx"
)

type Service struct {
	pkgNameRepo   repository.NameStore
	pkgBinaryRepo repository.BinaryRepository
	authRepo      repository.AuthRepository
	signerRepo    repository.SignerRepository
	denylistRepo  repository.DenylistRepository // nil when per-token revocation is not wired
	cfg           *conf.AyatoConfig
	catalog       *domain.RepositoryCatalog
	catalogErr    error
	// upstreamClient fetches upstream repo databases for the overlay/merge sync,
	// with the shared retry/timeout policy of pkg/httpx.
	upstreamClient *http.Client
	verifier       signatureTrust
}

//go:generate mockgen -source=service.go -destination=../test/mocks/service.go -package=mocks

// RepoReader exposes read-only queries over repos, arches, packages, and files.
type RepoReader interface {
	RepoNames() ([]string, error)
	Arches(repo string) ([]string, error)
	Repo(repo string) (*domain.PacmanRepo, error)
	Pkgs(repo, arch string) (*domain.PacmanPkgs, error)
	PkgDetail(repo, arch, pkgname string) (*domain.PacmanPackage, error)
	PkgFiles(repo, arch, pkg string) ([]string, error)
	RepoFileList(repo, arch string) ([]string, error)
	GetFileWithMeta(repoName, archName, name string) (platform.File, domain.FileMeta, error)
	SignedURL(repo string, arch string, name string) (string, error)
}

// Uploader mutates repo contents by publishing or removing package artifacts.
type Uploader interface {
	UploadFile(repo string, files *domain.UploadFiles) error
	UploadFiles(repo string, files []*domain.UploadFiles) error
	// ReconcileOrphans deletes package objects the repo db does not reference,
	// such as residue from an interrupted publication, older than olderThan;
	// dryRun reports without deleting.
	ReconcileOrphans(repo string, olderThan time.Duration, dryRun bool) ([]OrphanObject, error)
	RemovePkg(rname string, arch string, pkgname string) error
	// PresignUpload grants presigned staging PUTs for repo; CommitUpload then
	// validates and publishes from storage. Both return domain.ErrNotImplemented
	// when the blob backend has no staging capability (e.g. localfs).
	PresignUpload(repo string, files []domain.StagedFileRequest) (*domain.StagedUploadGrant, error)
	CommitUpload(repo, id string, entries []domain.StagedCommitEntry) error
}

// AdminService manages the admin allowlist. Adds take a numeric id, resolving a
// GitHub login to one via ResolveGitHubLogin so the outbound API call stays out
// of the handler layer.
type AdminService interface {
	IsAdmin(id int64) bool
	AddAdmin(id int64, login string) error
	RemoveAdmin(id int64) error
	ListAdmins() ([]domain.AllowedAdmin, error)
	SeedBootstrapAdmin(id int64) error
	ResolveGitHubLogin(ctx context.Context, login string) (int64, string, error)
}

// Revoker manages per-token revocation via the denylist. IsRevoked reports
// whether a token id (jti) was individually revoked; Revoke denylists one for
// ttl. Both are no-ops (false / configuration error) when no denylist is wired.
type Revoker interface {
	IsRevoked(jti string) (bool, error)
	IsSessionRevoked(sessionID string) (bool, error)
	Revoke(jti string, ttl time.Duration) error
	RevokeSession(sessionID string, ttl time.Duration) error
	ConsumeRefreshToken(jti string, ttl time.Duration) (bool, error)
}

// SignerRegistry manages worker signing keys. RegisterSigner accepts a worker
// public key certified by a configured master and persists it; ListSigners
// returns their fingerprints; UnregisterSigner revokes one by fingerprint.
type SignerRegistry interface {
	RegisterSigner(armoredPub []byte) (string, error)
	ListSigners() ([]string, error)
	UnregisterSigner(fingerprint string) error
}

// Lifecycle covers one-shot startup wiring that runs before serving.
type Lifecycle interface {
	InitAll() error
}

// Servicer is the composite the handler depends on today; the role interfaces
// above are the ISP seams that narrower handlers can adopt.
type Servicer interface {
	RepoReader
	Uploader
	Promoter
	Syncer
	AdminService
	SignerRegistry
	Revoker
	Lifecycle
}

func New(
	pkgNameRepo repository.NameStore,
	pkgBinaryRepo repository.BinaryRepository,
	authRepo repository.AuthRepository,
	signerRepo repository.SignerRepository,
	config *conf.AyatoConfig,
) *Service {
	catalog, _ := domain.NewRepositoryCatalog(nil, nil)
	s := &Service{
		pkgNameRepo:    pkgNameRepo,
		pkgBinaryRepo:  pkgBinaryRepo,
		authRepo:       authRepo,
		signerRepo:     signerRepo,
		cfg:            config,
		catalog:        catalog,
		upstreamClient: httpx.Default(),
	}
	if config == nil {
		return s
	}
	if configuredCatalog, err := config.RepositoryCatalog(); err != nil {
		s.catalogErr = err
	} else {
		s.catalog = configuredCatalog
	}
	s.verifier = loadSignatureTrust(config)
	return s
}

// InitAll validates fail-closed startup state and initializes each physical
// repository (including every tier).
func (s *Service) InitAll() error {
	if s.catalogErr != nil {
		return errors.WrapErr(s.catalogErr, "invalid repository catalog")
	}
	if s.verifier.err != nil {
		return s.verifier.err
	}
	repos := s.catalog.PhysicalNames()
	if len(repos) == 0 {
		slog.Warn("no repositories found in config, skipping initialization")
		return nil
	}
	slog.Debug("init all package repositories", "repos", repos)
	for _, repo := range repos {
		if err := s.initRepo(repo, s.signedDB(), nil); err != nil {
			return errors.WrapErr(err, fmt.Sprintf("failed to init repo %s", repo))
		}
	}
	return nil
}

// initRepo seeds declared and already-stored arches, backfilling arch=any
// packages when an architecture is first created.
func (s *Service) initRepo(
	repo string,
	useSignedDB bool,
	gnupgDir *string,
) error {
	for _, arch := range s.repoArches(repo) {
		if err := s.ensureArchSeeded(repo, arch, useSignedDB, gnupgDir); err != nil {
			return err
		}
	}
	return nil
}
