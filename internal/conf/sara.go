package conf

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

// SaraConfig configures the sara local AUR overlay daemon. sara serves an
// aurweb-compatible API on Port, resolving packages from trusted git overlays
// first and falling through to the real AUR.
type SaraConfig struct {
	Debug bool   `koanf:"debug"`
	Port  int    `koanf:"port"`
	Bind  string `koanf:"bind"` // listen address; default 127.0.0.1 (loopback only)

	// CacheDir holds the cloned overlay working trees. Defaults to
	// $XDG_CACHE_HOME/sara (or ~/.cache/sara).
	CacheDir string `koanf:"cache_dir"`
	// RefreshMinutes re-syncs overlays on this interval. 0 disables periodic
	// refresh (sync only happens at startup).
	RefreshMinutes int `koanf:"refresh_minutes"`
	// TrustStore is the path to the local trust store. Defaults to
	// $XDG_CONFIG_HOME/sara/trust.json.
	TrustStore string `koanf:"trust_store"`
	// EnforceMode gates resolution by trust: "warn" (default) annotates a
	// package whose approval is violated (e.g. maintainer changed); "enforce"
	// omits any package that is not approved, forcing `sara trust add` first.
	EnforceMode string `koanf:"enforce_mode"`

	Upstream UpstreamConfig  `koanf:"upstream"`
	Overlays []OverlayConfig `koanf:"overlays"`
	Ayato    []AyatoSource   `koanf:"ayato"`
}

// AyatoSource is a remote ayato instance federated as a package source. ayato
// hosts its own PKGBUILDs; sara ranks it above the upstream AUR but below local
// git overlays.
type AyatoSource struct {
	Name     string `koanf:"name"`
	URL      string `koanf:"url"` // ayato base URL, e.g. https://repo.example.com
	Priority int    `koanf:"priority,omitempty"`
	// PubKey is the base64 32-byte Ed25519 public key this source's catalog MUST
	// verify under — a HARD pin: a mismatch is fatal, never auto-accepted. Empty
	// requires Tofu or Insecure.
	PubKey string `koanf:"pubkey,omitempty"`
	// MaxAgeMinutes is sara's own staleness ceiling, independent of the catalog's
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

// ResolvedMaxAge is sara's staleness ceiling for this source: the configured
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

// UpstreamConfig is the real-AUR fallback for packages no overlay manages.
type UpstreamConfig struct {
	// Enabled turns on fallback. When false sara is a closed instance that only
	// answers for overlay packages.
	Enabled bool `koanf:"enabled"`
	// RPCURL is the upstream /rpc endpoint; empty uses the canonical AUR.
	RPCURL string `koanf:"rpc_url"`
	// GitBase overrides the git clone origin for redirects; empty derives it
	// from RPCURL.
	GitBase string `koanf:"git_base"`
	// UserAgent overrides the request User-Agent sent upstream.
	UserAgent string `koanf:"user_agent"`
}

// OverlayConfig is one trusted git repository hosting a PKGBUILD and .SRCINFO at
// its root (split packages allowed). Pin Ref to a commit so an upstream change
// to the overlay cannot silently alter what sara resolves.
type OverlayConfig struct {
	Name string `koanf:"name"`
	URL  string `koanf:"url"` // git clone URL; also the redirect target for git clone
	// Ref is the commit (recommended), tag, or branch to check out. Empty uses
	// the default branch HEAD, which is NOT pinned and should be avoided for
	// untrusted sources.
	Ref string `koanf:"ref,omitempty"`
	// Priority breaks ties when several overlays provide the same package name;
	// higher wins. Overlays always win over the upstream AUR.
	Priority int `koanf:"priority,omitempty"`
	// Maintainer is the synthetic maintainer label reported in RPC results.
	Maintainer string `koanf:"maintainer,omitempty"`
}

// ListenAddr returns the host:port sara binds to.
func (c *SaraConfig) ListenAddr() string {
	bind := c.Bind
	if bind == "" {
		bind = "127.0.0.1"
	}
	return fmt.Sprintf("%s:%d", bind, c.Port)
}

// ResolvedCacheDir returns the configured cache dir or a sensible default.
func (c *SaraConfig) ResolvedCacheDir() string {
	if c.CacheDir != "" {
		return c.CacheDir
	}
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "sara")
	}
	return filepath.Join(os.TempDir(), "sara")
}

// ServedRoot is where sara materializes pinned bare repos it serves directly
// (variant B), under the cache dir.
func (c *SaraConfig) ServedRoot() string {
	return filepath.Join(c.ResolvedCacheDir(), "served")
}

// AyatoPinStorePath is where TOFU-pinned ayato keys and the anti-rollback
// watermark live. It sits beside the trust store, not in the cache, so a cache
// wipe can't drop the pins (a downgrade vector).
func (c *SaraConfig) AyatoPinStorePath() string {
	return filepath.Join(filepath.Dir(c.ResolvedTrustStore()), "known_ayato.json")
}

// ResolvedTrustStore returns the configured trust-store path or a default under
// the user config dir.
func (c *SaraConfig) ResolvedTrustStore() string {
	if c.TrustStore != "" {
		return c.TrustStore
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "sara", "trust.json")
	}
	return filepath.Join(os.TempDir(), "sara-trust.json")
}

// AURGitBase returns the git origin used to clone AUR packages for auditing.
func (c *SaraConfig) AURGitBase() string {
	if c.Upstream.GitBase != "" {
		return strings.TrimRight(c.Upstream.GitBase, "/")
	}
	return "https://aur.archlinux.org"
}

// ResolvedEnforceMode returns "enforce" or the default "warn".
func (c *SaraConfig) ResolvedEnforceMode() string {
	if c.EnforceMode == "enforce" {
		return "enforce"
	}
	return "warn"
}

func LoadSaraConfig(flags *pflag.FlagSet, configFile string) (*SaraConfig, error) {
	if err := LoadEnv(); err != nil {
		slog.Error("Failed to load env", "error", err)
	}

	dirs := commonConfigDirs()
	files := []string{}
	if configFile != "" {
		files = append(files, configFile)
	} else {
		files = []string{"sara_config.json", "sara_config.toml", "sara_config.yaml"}
	}

	cfg, err := loadConfig[SaraConfig](dirs, files, flags, "SARA")
	if err != nil {
		return nil, err
	}
	if cfg.Port == 0 {
		cfg.Port = 10713
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate rejects overlays missing a name or URL and warns (does not fail) on
// unpinned refs, which are a supply-chain risk.
func (c *SaraConfig) Validate() error {
	names := map[string]bool{}
	for i, o := range c.Overlays {
		if o.Name == "" {
			return fmt.Errorf("overlays[%d]: name is required", i)
		}
		if o.URL == "" {
			return fmt.Errorf("overlay %q: url is required", o.Name)
		}
		if names[o.Name] {
			return fmt.Errorf("overlay %q: duplicate name", o.Name)
		}
		if o.Name == "overlay" || o.Name == "aur" {
			return fmt.Errorf("overlay name %q is reserved", o.Name)
		}
		names[o.Name] = true
		if o.Ref == "" {
			slog.Warn("overlay ref is not pinned; a moving branch lets the source change after review", "overlay", o.Name)
		}
	}

	ayatoNames := map[string]bool{}
	for i, a := range c.Ayato {
		if a.Name == "" {
			return fmt.Errorf("ayato[%d]: name is required", i)
		}
		if a.URL == "" {
			return fmt.Errorf("ayato %q: url is required", a.Name)
		}
		if ayatoNames[a.Name] {
			return fmt.Errorf("ayato %q: duplicate name", a.Name)
		}
		if a.Name == "overlay" || a.Name == "aur" {
			return fmt.Errorf("ayato name %q is reserved", a.Name)
		}
		ayatoNames[a.Name] = true
		if err := a.validate(); err != nil {
			return err
		}
	}

	// ayato federation persists TOFU pins and the anti-rollback watermark beside
	// the trust store. If trust_store is unset and we cannot find a user config
	// dir, ResolvedTrustStore silently falls back to a world-writable, reboot-wiped
	// temp path — an unacceptable home for trust anchors. Refuse to start and make
	// the operator set trust_store explicitly.
	if len(c.Ayato) > 0 && c.TrustStore == "" {
		if _, err := os.UserConfigDir(); err != nil {
			return fmt.Errorf("ayato federation needs a durable trust_store: set trust_store (no user config dir found, refusing the temp-dir fallback)")
		}
	}

	if c.EnforceMode != "" && c.EnforceMode != "warn" && c.EnforceMode != "enforce" {
		return fmt.Errorf("enforce_mode must be \"warn\" or \"enforce\", got %q", c.EnforceMode)
	}
	return nil
}
