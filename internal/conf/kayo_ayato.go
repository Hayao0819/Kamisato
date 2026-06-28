package conf

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"time"
)

// AyatoSource is a remote ayato instance federated as a package source. ayato
// hosts its own PKGBUILDs; kayo ranks it above the upstream AUR but below local
// git overlays.
type AyatoSource struct {
	Name     string `koanf:"name"`
	URL      string `koanf:"url"` // ayato base URL, e.g. https://repo.example.com
	Priority int    `koanf:"priority,omitempty"`
	// PubKey is the base64 32-byte Ed25519 public key this source's catalog MUST
	// verify under — a HARD pin: a mismatch is fatal, never auto-accepted. Empty
	// requires Tofu or Insecure.
	PubKey string `koanf:"pubkey,omitempty"`
	// MaxAgeMinutes is kayo's own staleness ceiling, independent of the catalog's
	// signed ExpiresAt, so a misconfigured ayato can't hand out an unbounded TTL.
	// 0 with a pinned/tofu key uses a safe default (24h).
	MaxAgeMinutes int `koanf:"max_age_minutes,omitempty"`
	// Trust selects what a verified signature buys: "review" (default) still
	// routes every package through the trust store; "delegate" treats the
	// authenticated catalog as vouched and bypasses review. Requires a pinned key.
	Trust string `koanf:"trust,omitempty"`
	// Tofu allows trust-on-first-use (pin /pubkey on first sync) when PubKey is
	// empty. Off by default: an unpinned source is otherwise a config error.
	Tofu bool `koanf:"tofu,omitempty"`
	// Insecure disables verification entirely (unsigned/legacy ayato). The only
	// way to accept an unsigned catalog; off by default.
	Insecure bool `koanf:"insecure,omitempty"`
}

const defaultAyatoMaxAge = 24 * time.Hour

// validate enforces secure-by-default federation: a source is verified unless it
// explicitly opts out with insecure, and delegation only rides a hard-pinned key.
func (a AyatoSource) validate() error {
	switch a.Trust {
	case "", "review", "delegate":
	default:
		return fmt.Errorf("ayato %q: trust must be \"review\" or \"delegate\", got %q", a.Name, a.Trust)
	}
	if a.Insecure {
		if a.PubKey != "" || a.Tofu || a.Trust == "delegate" {
			return fmt.Errorf("ayato %q: insecure cannot combine with pubkey/tofu/delegate", a.Name)
		}
		return nil
	}
	if a.PubKey == "" && !a.Tofu {
		return fmt.Errorf("ayato %q: set pubkey to pin a key, or tofu to trust on first use (or insecure to opt out)", a.Name)
	}
	if a.PubKey != "" {
		key, err := base64.StdEncoding.DecodeString(a.PubKey)
		if err != nil {
			return fmt.Errorf("ayato %q: pubkey is not valid base64: %w", a.Name, err)
		}
		if len(key) != ed25519.PublicKeySize {
			return fmt.Errorf("ayato %q: pubkey must be %d bytes, got %d", a.Name, ed25519.PublicKeySize, len(key))
		}
	}
	if a.Trust == "delegate" && a.PubKey == "" {
		return fmt.Errorf("ayato %q: trust \"delegate\" requires a pinned pubkey (tofu is too weak to bypass review)", a.Name)
	}
	if a.MaxAgeMinutes < 0 {
		return fmt.Errorf("ayato %q: max_age_minutes cannot be negative", a.Name)
	}
	return nil
}

// ResolvedMaxAge is kayo's staleness ceiling for this source: the configured
// value, or a 24h floor so a verified-but-stale catalog can't linger forever.
func (a AyatoSource) ResolvedMaxAge() time.Duration {
	if a.MaxAgeMinutes > 0 {
		return time.Duration(a.MaxAgeMinutes) * time.Minute
	}
	return defaultAyatoMaxAge
}

// Delegated reports whether a verified catalog from this source bypasses the
// trust store. Only a hard-pinned, non-insecure source with trust="delegate"
// qualifies; validate guarantees those preconditions.
func (a AyatoSource) Delegated() bool {
	return !a.Insecure && a.PubKey != "" && a.Trust == "delegate"
}
