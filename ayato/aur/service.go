package aur

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/Hayao0819/Kamisato/internal/kayoproto"
)

// SourceManager is the subset of *Backend the service drives.
type SourceManager interface {
	Register(ctx context.Context, gitURL, ref, maintainer string) (pkgbase string, names []string, err error)
	Remove(ctx context.Context, pkgbase string) error
	List(ctx context.Context) ([]string, error)
	Catalog(ctx context.Context) (kayoproto.Catalog, error)
}

// Service is the gin-free backend for AUR source management and the kayo-facing
// catalog; the gin glue lives in ayato/handler.
type Service struct {
	sm     SourceManager
	signer *CatalogSigner

	// The public catalog is unauthenticated and rebuilds from hundreds of KV
	// reads per request, so the built envelope is cached for cacheTTL. Writes are
	// not invalidated; they wait out the TTL.
	cacheTTL time.Duration
	cacheMu  sync.Mutex
	cached   *kayoproto.CatalogEnvelope
	cacheExp time.Time
}

// NewService builds the AUR service. cacheTTL bounds how long a built envelope is
// reused.
func NewService(sm SourceManager, cacheTTL time.Duration) *Service {
	return &Service{sm: sm, cacheTTL: cacheTTL}
}

// WithSigner enables catalog signing. A nil signer serves the catalog unsigned
// (legacy); kayo refuses that for any pinned source.
func (s *Service) WithSigner(signer *CatalogSigner) *Service {
	s.signer = signer
	return s
}

// Register registers an external PKGBUILD repo, returning its pkgbase and the
// package names it provides.
func (s *Service) Register(ctx context.Context, gitURL, ref, maintainer string) (pkgbase string, names []string, err error) {
	return s.sm.Register(ctx, gitURL, ref, maintainer)
}

// List returns every registered pkgbase.
func (s *Service) List(ctx context.Context) ([]string, error) {
	return s.sm.List(ctx)
}

// Remove deregisters a pkgbase and drops its derived metadata.
func (s *Service) Remove(ctx context.Context, pkgbase string) error {
	return s.sm.Remove(ctx, pkgbase)
}

// Envelope returns the catalog as a signed envelope, served from cache while it
// is still fresh so repeated hits don't re-fan-out to KV. A nil signer yields an
// unsigned ("none") envelope; kayo verifies the signature instead of credentials.
func (s *Service) Envelope(ctx context.Context) (kayoproto.CatalogEnvelope, error) {
	if env := s.cachedEnvelope(); env != nil {
		return *env, nil
	}

	cat, err := s.sm.Catalog(ctx)
	if err != nil {
		return kayoproto.CatalogEnvelope{}, err
	}

	var env kayoproto.CatalogEnvelope
	if s.signer == nil {
		payload, mErr := json.Marshal(kayoproto.SignedPayload{IssuedAt: time.Now().UTC(), Catalog: cat})
		if mErr != nil {
			return kayoproto.CatalogEnvelope{}, mErr
		}
		env = kayoproto.CatalogEnvelope{Payload: payload, Alg: "none"}
	} else {
		env, err = s.signer.Sign(cat)
		if err != nil {
			return kayoproto.CatalogEnvelope{}, err
		}
	}

	s.storeEnvelope(env)
	return env, nil
}

// SignerEnabled reports whether catalog signing is configured.
func (s *Service) SignerEnabled() bool { return s.signer != nil }

// SignerKeyID returns the catalog signing key id, or "" when signing is off.
func (s *Service) SignerKeyID() string {
	if s.signer == nil {
		return ""
	}
	return s.signer.KeyID()
}

// SignerPublicKeyB64 returns the base64 catalog signing public key, or "" when
// signing is off.
func (s *Service) SignerPublicKeyB64() string {
	if s.signer == nil {
		return ""
	}
	return s.signer.PublicKeyB64()
}

func (s *Service) cachedEnvelope() *kayoproto.CatalogEnvelope {
	if s.cacheTTL <= 0 {
		return nil
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if s.cached != nil && time.Now().Before(s.cacheExp) {
		return s.cached
	}
	return nil
}

func (s *Service) storeEnvelope(env kayoproto.CatalogEnvelope) {
	if s.cacheTTL <= 0 {
		return
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.cached = &env
	s.cacheExp = time.Now().Add(s.cacheTTL)
}
