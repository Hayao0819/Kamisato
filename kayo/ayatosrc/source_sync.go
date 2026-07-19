package ayatosrc

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/Hayao0819/Kamisato/internal/client"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/kayoproto"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

// Sync fetches the catalog, verifies it (signature + freshness) unless insecure,
// and only then swaps in a fresh index. Any verification error returns before the
// swap, so the last-good catalog survives (fail-closed).
func (s *Source) Sync(ctx context.Context) error {
	body, err := s.catalog.Fetch(ctx)
	if err != nil {
		return errors.WrapErr(err, "ayato catalog: "+s.name)
	}

	var env kayoproto.CatalogEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return errors.WrapErr(err, "ayato catalog decode: "+s.name)
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
			return errors.WrapErr(err, "ayato payload decode: "+s.name)
		}
	} else {
		v, err := s.resolveVerifier(ctx)
		if err != nil {
			return err
		}
		if legacy || env.Alg != "ed25519" || env.Signature == "" {
			return errors.NewErrf("ayato %s: catalog is unsigned but a key is pinned (refusing downgrade)", s.name)
		}
		if err := v.VerifyPayload(env.Payload, env.Signature); err != nil {
			return errors.WrapErr(err, "ayato "+s.name+" (key_id hint "+env.KeyID+")")
		}
		if err := json.Unmarshal(env.Payload, &signed); err != nil {
			return errors.WrapErr(err, "ayato payload decode: "+s.name)
		}
		s.mu.RLock()
		watermark := s.lastIssued
		s.mu.RUnlock()
		if err := v.CheckFreshness(signed.IssuedAt, signed.ExpiresAt, watermark); err != nil {
			return errors.WrapErr(err, "ayato "+s.name)
		}
		verified = true
		if s.pins != nil {
			// The watermark MUST reach disk before the index swaps: otherwise the served
			// catalog advances to T2 while the persisted floor stays at T1, so a restart
			// would re-accept a replayed T1<t<=T2 catalog. Persist failure is fatal (fail-closed).
			if perr := s.pins.SetLastIssued(s.name, signed.IssuedAt); perr != nil {
				return errors.WrapErr(perr, "ayato "+s.name+": persist rollback watermark")
			}
		}
	}

	cat := signed.Catalog
	index := make(map[string]aurweb.Pkg, len(cat.Packages))
	for _, p := range cat.Packages {
		index[p.Name] = p
	}
	s.Replace(index, cat.Sources)

	s.mu.Lock()
	s.lastVerified = verified
	s.expiresAt = signed.ExpiresAt
	if verified && signed.IssuedAt.After(s.lastIssued) {
		s.lastIssued = signed.IssuedAt
	}
	s.mu.Unlock()
	return nil
}

// Verified reports whether the served catalog is signature-verified AND still fresh.
// Freshness matters because a failed re-sync keeps the last-good catalog (fail-closed):
// without an expiry, an attacker who blocks kayo->ayato could freeze an aging catalog
// and keep its delegation alive forever. The bound is the tighter of the catalog's
// signed ExpiresAt and kayo's maxAge, so short-lived attestations revoke continuously.
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
	if !s.trustOnFirstUse || s.pins == nil {
		return nil, errors.NewErrf("ayato %s: no pinned public key (set pubkey, or enable trust_on_first_use/insecure)", s.name)
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

// FetchPubkey retrieves the catalog-signing public key an ayato base URL advertises,
// for an operator converting a TOFU pin into a hard config pin. The key is
// unauthenticated on its own — verify the key_id out of band before trusting it.
func FetchPubkey(ctx context.Context, baseURL string) (pubkey, keyID string, err error) {
	catalog, err := client.NewCatalog(baseURL)
	if err != nil {
		return "", "", err
	}
	body, err := catalog.FetchPublicKey(ctx)
	if err != nil {
		return "", "", err
	}
	var r struct {
		Pubkey string `json:"pubkey"`
		KeyID  string `json:"key_id"`
	}
	if err := json.Unmarshal(body, &r); err != nil || r.Pubkey == "" {
		return "", "", errors.NewErrf("ayato %s: could not read /pubkey", baseURL)
	}
	return r.Pubkey, r.KeyID, nil
}

func (s *Source) fetchPubkey(ctx context.Context) (string, error) {
	body, err := s.catalog.FetchPublicKey(ctx)
	if err != nil {
		return "", errors.WrapErr(err, "ayato catalog public key: "+s.name)
	}
	var r struct {
		Pubkey string `json:"pubkey"`
	}
	if err := json.Unmarshal(body, &r); err != nil || r.Pubkey == "" {
		return "", errors.NewErrf("ayato %s: could not read /pubkey for TOFU", s.name)
	}
	return r.Pubkey, nil
}
