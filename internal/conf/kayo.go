package conf

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/pflag"
)

// KayoConfig configures the kayo local AUR overlay daemon. kayo serves an
// aurweb-compatible API on Addr, resolving packages from trusted git overlays
// first and falling through to the real AUR.
type KayoConfig struct {
	Debug bool   `koanf:"debug"`
	Addr  string `koanf:"addr"` // listen address (host:port); default 127.0.0.1:10713 (loopback only)

	// CacheDir holds the cloned overlay working trees. Defaults to
	// $XDG_CACHE_HOME/kayo (or ~/.cache/kayo).
	CacheDir string `koanf:"cache_dir"`
	// RefreshMinutes re-syncs overlays on this interval. 0 disables periodic
	// refresh (sync only happens at startup).
	RefreshMinutes int `koanf:"refresh_minutes"`
	// TrustStore is the path to the local trust store. Defaults to
	// $XDG_CONFIG_HOME/kayo/trust.json.
	TrustStore string `koanf:"trust_store"`
	// YayCacheDir is yay's clone-cache root (defaults to $XDG_CACHE_HOME/yay). The
	// build-time pin check reads each pkgbase's checked-out commit from here and
	// flags a checkout that drifted off the approved commit.
	YayCacheDir string `koanf:"yay_cache_dir"`
	// EnforceMode gates resolution by trust: "warn" (default) annotates a
	// package whose approval is violated (e.g. maintainer changed); "enforce"
	// omits any package that is not approved, forcing `kayo trust add` first.
	EnforceMode string `koanf:"enforce_mode"`

	Upstream UpstreamConfig  `koanf:"upstream"`
	Overlays []OverlayConfig `koanf:"overlays"`
	Ayato    []AyatoSource   `koanf:"ayato"`
	LLM      LLMConfig       `koanf:"llm,omitempty"`
}

// LLMConfig configures the optional LLM advisory pass over a PKGBUILD, run only by
// the human-driven audit/trust commands (never the resolve path) and advisory, not a
// gate. The API key comes from the provider's standard env var, never config.
type LLMConfig struct {
	// Enabled runs the advisory by default on audit/trust add. The --llm flag can
	// force it on for a single run when this is false.
	Enabled bool `koanf:"enabled"`
	// Provider is anthropic (default), openai, or ollama.
	Provider string `koanf:"provider,omitempty"`
	Model    string `koanf:"model,omitempty"`
	// BaseURL overrides the provider endpoint (and is the ollama server URL).
	BaseURL string `koanf:"base_url,omitempty"`
}

// UpstreamConfig is the real-AUR fallback for packages no overlay manages.
type UpstreamConfig struct {
	// Enabled turns on fallback. When false kayo is a closed instance that only
	// answers for overlay packages.
	Enabled bool `koanf:"enabled"`
	// RPCURL is the upstream /rpc endpoint; empty uses the canonical AUR.
	RPCURL string `koanf:"rpc_url"`
	// GitBase overrides the git clone origin for redirects; empty derives it
	// from RPCURL.
	GitBase   string `koanf:"git_base"`
	UserAgent string `koanf:"user_agent"`
}

// OverlayConfig is one trusted git repository hosting a PKGBUILD and .SRCINFO at
// its root (split packages allowed). Pin Ref to a commit so an upstream change
// to the overlay cannot silently alter what kayo resolves.
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

func LoadKayoConfig(flags *pflag.FlagSet, configFile string) (*KayoConfig, error) {
	loadDotEnv()
	return LoadTyped[KayoConfig](
		commonConfigDirs(),
		configFileNames(configFile, "kayo_config"),
		flags,
		"KAYO",
		func(c *KayoConfig) {
			if c.Addr == "" {
				c.Addr = "127.0.0.1:10713"
			}
		},
	)
}

// Validate rejects overlays missing a name or URL and warns (does not fail) on
// unpinned refs, which are a supply-chain risk.
func (c *KayoConfig) Validate() error {
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

	// ayato federation stores TOFU pins and the anti-rollback watermark beside the
	// trust store. Without trust_store and a user config dir, ResolvedTrustStore falls
	// back to a world-writable, reboot-wiped temp path — unacceptable for trust
	// anchors — so refuse to start.
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
