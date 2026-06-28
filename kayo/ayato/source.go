// Package ayato makes a remote ayato instance act as a kayo package source. It
// fetches the instance's catalog (its own-hosted PKGBUILDs plus their git URLs)
// and implements aurweb.Backend so kayo can federate ayato alongside local git
// overlays and the upstream AUR.
package ayato

import (
	"cmp"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Hayao0819/Kamisato/internal/kayoproto"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

const (
	catalogPath     = "/api/unstable/aur/catalog"
	pubkeyPath      = "/api/unstable/aur/pubkey"
	maxCatalogBytes = 32 << 20
)

// Options configures a Source.
type Options struct {
	Name, BaseURL string
	PubKey        string        // explicit hard pin (base64); "" => TOFU or Insecure
	MaxAge        time.Duration // freshness ceiling; must be > 0 unless Insecure
	Insecure      bool          // accept an unsigned catalog (escape hatch)
	Tofu          bool          // allow trust-on-first-use when PubKey is empty
	Pins          *PinStore
}

// Source is one ayato instance, refreshed by Sync. A catalog only swaps in after
// its signature and freshness verify (unless Insecure), so the index never holds
// unverified data.
type Source struct {
	name     string
	base     string
	client   *http.Client
	maxAge   time.Duration
	insecure bool
	tofu     bool
	pins     *PinStore

	mu           sync.RWMutex
	index        map[string]aurweb.Pkg
	sources      map[string]string
	names        []string
	verifier     *Verifier // set when explicitly pinned
	lastIssued   time.Time // anti-rollback watermark (in-memory)
	expiresAt    time.Time // served catalog's signed expiry (zero if none)
	lastVerified bool      // last Sync passed signature+freshness
}

// New builds a Source. An explicit PubKey is a hard pin; an empty PubKey requires
// either Tofu or Insecure.
func New(o Options) (*Source, error) {
	s := &Source{
		name:     o.Name,
		base:     strings.TrimRight(o.BaseURL, "/"),
		client:   &http.Client{Timeout: 15 * time.Second},
		maxAge:   o.MaxAge,
		insecure: o.Insecure,
		tofu:     o.Tofu,
		pins:     o.Pins,
		index:    map[string]aurweb.Pkg{},
		sources:  map[string]string{},
	}
	if o.PubKey != "" {
		v, err := NewVerifier(o.PubKey, o.MaxAge)
		if err != nil {
			return nil, err
		}
		s.verifier = v
	}
	if o.Pins != nil { // restore the rollback watermark across restarts
		if p, ok := o.Pins.Get(o.Name); ok {
			s.lastIssued = p.LastIssued
		}
	}
	return s, nil
}

// Sync fetches the catalog, verifies it (signature + freshness) unless insecure,
// and only then swaps in a fresh index. Any verification error returns before the
// swap, so the last-good catalog survives (fail-closed).
func (s *Source) Sync(ctx context.Context) error {
	body, err := s.fetch(ctx, catalogPath)
	if err != nil {
		return err
	}

	var env kayoproto.CatalogEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return utils.WrapErr(err, "ayato catalog decode: "+s.name)
	}
	legacy := len(env.Payload) == 0

	var signed kayoproto.SignedPayload
	verified := false
	if s.insecure {
		if legacy {
			err = json.Unmarshal(body, &signed.Catalog)
		} else {
			err = json.Unmarshal(env.Payload, &signed)
		}
		if err != nil {
			return utils.WrapErr(err, "ayato payload decode: "+s.name)
		}
	} else {
		v, err := s.resolveVerifier(ctx)
		if err != nil {
			return err
		}
		if legacy || env.Alg != "ed25519" || env.Signature == "" {
			return utils.NewErrf("ayato %s: catalog is unsigned but a key is pinned (refusing downgrade)", s.name)
		}
		if err := v.VerifyPayload(env.Payload, env.Signature); err != nil {
			return utils.WrapErr(err, "ayato "+s.name+" (key_id hint "+env.KeyID+")")
		}
		if err := json.Unmarshal(env.Payload, &signed); err != nil {
			return utils.WrapErr(err, "ayato payload decode: "+s.name)
		}
		s.mu.RLock()
		watermark := s.lastIssued
		s.mu.RUnlock()
		if err := v.CheckFreshness(signed.IssuedAt, signed.ExpiresAt, watermark); err != nil {
			return utils.WrapErr(err, "ayato "+s.name)
		}
		verified = true
		if s.pins != nil {
			// The watermark MUST reach disk before the index swaps: otherwise the
			// served catalog advances to T2 while the persisted floor stays at T1,
			// and a restart would re-accept a replayed T1<t<=T2 catalog. Treat a
			// persist failure as fatal and keep the last-good catalog (fail-closed).
			if perr := s.pins.SetLastIssued(s.name, signed.IssuedAt); perr != nil {
				return utils.WrapErr(perr, "ayato "+s.name+": persist rollback watermark")
			}
		}
	}

	cat := signed.Catalog
	index := make(map[string]aurweb.Pkg, len(cat.Packages))
	names := make([]string, 0, len(cat.Packages))
	for _, p := range cat.Packages {
		index[p.Name] = p
		names = append(names, p.Name)
	}
	slices.Sort(names)

	s.mu.Lock()
	s.index, s.sources, s.names, s.lastVerified = index, cat.Sources, names, verified
	s.expiresAt = signed.ExpiresAt
	if verified && signed.IssuedAt.After(s.lastIssued) {
		s.lastIssued = signed.IssuedAt
	}
	s.mu.Unlock()
	return nil
}

// Verified reports whether the currently-served catalog is signature-verified
// AND still fresh. The freshness bound matters because a failed re-sync leaves
// the last-good catalog in place (fail-closed): without it, an attacker who
// blocks kayo->ayato could freeze a verified-but-aging catalog and keep its
// delegation bypass alive forever. It falls closed at the tighter of the
// catalog's own signed ExpiresAt and kayo's maxAge ceiling — the same bound
// CheckFreshness applies at sync time — so a source using short-lived
// attestations for revocation is honored continuously, not just at the swap.
func (s *Source) Verified() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.lastVerified {
		return false
	}
	now := time.Now()
	if !s.expiresAt.IsZero() && now.After(s.expiresAt) {
		return false
	}
	if s.maxAge > 0 && now.Sub(s.lastIssued) > s.maxAge {
		return false
	}
	return true
}

// resolveVerifier returns the verifier to check this catalog against: the
// explicit pin, the stored TOFU pin, or (on first contact) the key fetched from
// /pubkey. A rotation under TOFU surfaces as a signature failure — never as a
// silent re-pin off the unauthenticated key_id hint.
func (s *Source) resolveVerifier(ctx context.Context) (*Verifier, error) {
	if s.verifier != nil {
		return s.verifier, nil
	}
	if !s.tofu || s.pins == nil {
		return nil, utils.NewErrf("ayato %s: no pinned public key (set pubkey, or enable tofu/insecure)", s.name)
	}
	if p, ok := s.pins.Get(s.name); ok && p.PubKey != "" {
		return NewVerifier(p.PubKey, s.maxAge)
	}
	pub, err := s.fetchPubkey(ctx) // first contact
	if err != nil {
		return nil, err
	}
	v, err := NewVerifier(pub, s.maxAge)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if err := s.pins.Put(s.name, pin{PubKey: pub, KeyID: v.KeyID(), FirstSeen: now, LastSeen: now}); err != nil {
		return nil, err
	}
	slog.Warn("pinned ayato key on first use (TOFU); pin it in config for a strong anchor", "name", s.name, "key_id", v.KeyID())
	return v, nil
}

func (s *Source) fetch(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, utils.WrapErr(err, "ayato request "+path+": "+s.name)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, utils.NewErrf("ayato %s: %s status %d", s.name, path, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxCatalogBytes))
}

// FetchPubkey retrieves the catalog-signing public key an ayato base URL
// advertises, for an operator converting a TOFU pin into a hard config pin.
// The key is unauthenticated on its own — verify the key_id out of band before
// pasting it into config.
func FetchPubkey(ctx context.Context, baseURL string) (pubkey, keyID string, err error) {
	s, err := New(Options{Name: "fetch", BaseURL: baseURL, Insecure: true})
	if err != nil {
		return "", "", err
	}
	body, err := s.fetch(ctx, pubkeyPath)
	if err != nil {
		return "", "", err
	}
	var r struct {
		Pubkey string `json:"pubkey"`
		KeyID  string `json:"key_id"`
	}
	if err := json.Unmarshal(body, &r); err != nil || r.Pubkey == "" {
		return "", "", utils.NewErrf("ayato %s: could not read /pubkey", baseURL)
	}
	return r.Pubkey, r.KeyID, nil
}

func (s *Source) fetchPubkey(ctx context.Context) (string, error) {
	body, err := s.fetch(ctx, pubkeyPath)
	if err != nil {
		return "", err
	}
	var r struct {
		Pubkey string `json:"pubkey"`
	}
	if err := json.Unmarshal(body, &r); err != nil || r.Pubkey == "" {
		return "", utils.NewErrf("ayato %s: could not read /pubkey for TOFU", s.name)
	}
	return r.Pubkey, nil
}

func (s *Source) Info(_ context.Context, requested []string) ([]aurweb.Pkg, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []aurweb.Pkg
	for _, n := range requested {
		if p, ok := s.index[n]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *Source) Search(_ context.Context, by aurweb.By, arg string) ([]aurweb.Pkg, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []aurweb.Pkg
	for _, p := range s.index {
		if aurweb.Match(p, by, arg) {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(a, b aurweb.Pkg) int { return cmp.Compare(a.Name, b.Name) })
	return out, nil
}

func (s *Source) Suggest(_ context.Context, arg string, pkgbase bool) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pool := s.names
	if pkgbase {
		seen := map[string]bool{}
		pool = nil
		for _, p := range s.index {
			if !seen[p.PackageBase] {
				seen[p.PackageBase] = true
				pool = append(pool, p.PackageBase)
			}
		}
		slices.Sort(pool)
	}

	var out []string
	for _, n := range pool {
		if strings.HasPrefix(n, arg) {
			out = append(out, n)
			if len(out) >= 20 {
				break
			}
		}
	}
	return out, nil
}

func (s *Source) All(_ context.Context) ([]aurweb.Pkg, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]aurweb.Pkg, 0, len(s.index))
	for _, p := range s.index {
		out = append(out, p)
	}
	return out, nil
}

func (s *Source) SourceURL(_ context.Context, pkgbase string) (string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if u, ok := s.sources[pkgbase]; ok {
		return u, true, nil
	}
	return "", false, nil
}
