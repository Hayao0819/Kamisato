package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/blob"
	"github.com/Hayao0819/Kamisato/ayato/domain"
)

type Service struct {
	pkgNameRepo   NameStore
	pkgBinaryRepo BinaryRepository
	authRepo      AuthRepository
	signerRepo    SignerRepository
	denylistRepo  DenylistRepository // nil when per-token revocation is not wired
	settings      Settings
	catalog       *domain.RepositoryCatalog
	catalogErr    error
	// upstreamClient fetches upstream repo databases for the overlay/merge sync.
	upstreamClient *http.Client
	verifier       signatureTrust
}

type Settings struct {
	Catalog                    *domain.RepositoryCatalog
	CatalogError               error
	RequireSign                bool
	RequireBuildinfoProvenance bool
	ExpectedBuildDir           string
	ProtectedNames             []string
	MaxBatchPackages           int
	MaxPackageSize             int
	SignDatabase               bool
	VerificationKeyring        string
	TrustedVerificationKeys    []string
	MasterVerificationKeys     []string
}

func (settings Settings) normalized() Settings {
	if settings.Catalog == nil {
		settings.Catalog, _ = domain.NewRepositoryCatalog(nil, nil)
	}
	if settings.ExpectedBuildDir == "" {
		settings.ExpectedBuildDir = "/build"
	}
	return settings
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
	PkgSignature(repo, arch, pkgname string) (*domain.PackageSignature, error)
	RepoFileList(repo, arch string) ([]string, error)
	GetFileWithMeta(repoName, archName, name string) (blob.File, domain.FileMeta, error)
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
	pkgNameRepo NameStore,
	pkgBinaryRepo BinaryRepository,
	authRepo AuthRepository,
	signerRepo SignerRepository,
	settings Settings,
) *Service {
	settings = settings.normalized()
	s := &Service{
		pkgNameRepo:    pkgNameRepo,
		pkgBinaryRepo:  pkgBinaryRepo,
		authRepo:       authRepo,
		signerRepo:     signerRepo,
		settings:       settings,
		catalog:        settings.Catalog,
		catalogErr:     settings.CatalogError,
		upstreamClient: &http.Client{Timeout: 30 * time.Second},
	}
	s.verifier = loadSignatureTrust(settings)
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
