package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/pkg/httpx"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
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

	// Signature trust roots: baseEntities from verify.keyring, trustedFprs from
	// verify.trusted_keys (allowlist), masterEntities from verify.master_keys
	// (which certify workers registered at runtime via RegisterSigner).
	// verifierErr is a fail-closed startup error surfaced by InitAll.
	baseEntities   openpgp.EntityList
	masterEntities openpgp.EntityList
	trustedFprs    []string
	verifyRoot     bool
	verifierErr    error
}

//go:generate mockgen -source=service.go -destination=../test/mocks/service.go -package=mocks

// RepoReader exposes read-only queries over repos, arches, packages, and files.
type RepoReader interface {
	RepoNames() ([]string, error)
	Arches(repo string) ([]string, error)
	Repo(repo string) (*domain.PacmanRepo, error)
	Pkgs(repo, arch string) (*domain.PacmanPkgs, error)
	PkgDetail(repo, arch, pkgbase string) (*raiou.PKGINFO, error)
	PkgFiles(repo, arch, pkg string) ([]string, error)
	RepoFileList(repo, arch string) ([]string, error)
	GetFileWithMeta(repoName, archName, name string) (stream.File, domain.FileMeta, error)
	SignedURL(repo string, arch string, name string) (string, error)
}

// Uploader mutates repo contents by publishing or removing package artifacts.
type Uploader interface {
	UploadFile(repo string, files *domain.UploadFiles) error
	UploadFiles(repo string, files []*domain.UploadFiles) error
	// PresignUploads issues a direct-to-store PUT URL per file; FinalizeUploads
	// validates and registers the objects the client PUT via those URLs.
	PresignUploads(repo string, filenames []string) (map[string]string, error)
	FinalizeUploads(repo string, pkgFilenames []string) error
	// ReconcileOrphans deletes package objects the repo db does not reference —
	// the residue of a presigned upload PUT but never finalized — older than
	// olderThan; dryRun reports without deleting.
	ReconcileOrphans(repo string, olderThan time.Duration, dryRun bool) ([]OrphanObject, error)
	RemovePkg(rname string, arch string, pkgname string) error
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

func New(pkgNameRepo repository.NameStore, pkgBinaryRepo repository.BinaryRepository, authRepo repository.AuthRepository, signerRepo repository.SignerRepository, config *conf.AyatoConfig) *Service {
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

	s.trustedFprs = config.Verify.TrustedKeys

	// The keyring is public-key material, separate from the signing private key.
	if config.Verify.Keyring != "" {
		data, err := os.ReadFile(config.Verify.Keyring)
		if err == nil {
			s.baseEntities, err = sign.ReadEntities(data)
		}
		if err != nil {
			s.verifierErr = fmt.Errorf("load package-signature keyring: %w", err)
			slog.Error("failed to load package-signature keyring", "path", config.Verify.Keyring, "err", err)
		} else {
			slog.Info("package-signature verification enabled", "keyring", config.Verify.Keyring, "trusted_keys", len(config.Verify.TrustedKeys))
		}
	}

	for i, armored := range config.Verify.MasterKeys {
		ents, err := sign.ReadEntities([]byte(armored))
		if err != nil {
			s.verifierErr = fmt.Errorf("parse verify.master_keys[%d]: %w", i, err)
			slog.Error("failed to parse master key", "index", i, "err", err)
			continue
		}
		s.masterEntities = append(s.masterEntities, ents...)
	}

	// A trust root exists if a keyring is configured or a master can certify
	// runtime-registered workers; without one we cannot fail closed.
	s.verifyRoot = config.Verify.Keyring != "" || len(config.Verify.MasterKeys) > 0
	if config.RequireSign && !s.verifyRoot && s.verifierErr == nil {
		s.verifierErr = fmt.Errorf("require_sign is enabled but neither verify.keyring nor verify.master_keys is configured; cannot fail closed without a trust root")
	}

	return s
}

// verifyKeyring composes the trust root for package verification: the configured
// keyring plus every registered worker key. It returns nil when no entity is
// available, so a present signature with no trust root is rejected by the caller.
func (s *Service) verifyKeyring() (*sign.Keyring, error) {
	entities := slices.Clone(s.baseEntities)
	trusted := slices.Clone(s.trustedFprs)
	if s.signerRepo != nil {
		regs, err := s.signerRepo.ListSigners()
		if err != nil {
			return nil, err
		}
		for _, armored := range regs {
			ents, perr := sign.ReadEntities(armored)
			if perr != nil {
				slog.Warn("skipping unparseable registered signer key", "err", perr)
				continue
			}
			entities = append(entities, ents...)
			// Registered workers are already gated by master certification, so
			// they must pass even when verify.trusted_keys pins the base keyring.
			if len(s.trustedFprs) > 0 {
				for _, e := range ents {
					trusted = append(trusted, fmt.Sprintf("%X", e.PrimaryKey.Fingerprint))
				}
			}
		}
	}
	if len(entities) == 0 {
		return nil, nil
	}
	return sign.NewKeyring(entities, trusted), nil
}

func (s *Service) UnregisterSigner(fingerprint string) error {
	if s.signerRepo == nil {
		return fmt.Errorf("signer registration is not available")
	}
	return s.signerRepo.DeleteSigner(fingerprint)
}

// RegisterSigner persists a worker public key after confirming it is certified by
// a configured master, returning its uppercase-hex fingerprint.
func (s *Service) RegisterSigner(armoredPub []byte) (string, error) {
	if s.signerRepo == nil {
		return "", fmt.Errorf("signer registration is not available")
	}
	ents, err := sign.ReadEntities(armoredPub)
	if err != nil {
		return "", fmt.Errorf("parse signer key: %w", err)
	}
	if len(ents) != 1 {
		return "", fmt.Errorf("expected exactly one signer key, got %d", len(ents))
	}
	worker := ents[0]
	if len(s.masterEntities) == 0 {
		return "", fmt.Errorf("no verify.master_keys configured to certify a worker key")
	}
	certified := false
	for _, master := range s.masterEntities {
		if sign.CertifiedBy(worker, master) == nil {
			certified = true
			break
		}
	}
	if !certified {
		return "", fmt.Errorf("worker key is not certified by any configured master key")
	}
	fpr := fmt.Sprintf("%X", worker.PrimaryKey.Fingerprint)
	if err := s.signerRepo.AddSigner(fpr, armoredPub); err != nil {
		return "", err
	}
	slog.Info("registered worker signing key", "fingerprint", fpr)
	return fpr, nil
}

func (s *Service) ListSigners() ([]string, error) {
	if s.signerRepo == nil {
		return nil, nil
	}
	regs, err := s.signerRepo.ListSigners()
	if err != nil {
		return nil, err
	}
	var fprs []string
	for _, armored := range regs {
		ents, perr := sign.ReadEntities(armored)
		if perr != nil || len(ents) == 0 {
			continue
		}
		fprs = append(fprs, fmt.Sprintf("%X", ents[0].PrimaryKey.Fingerprint))
	}
	return fprs, nil
}
