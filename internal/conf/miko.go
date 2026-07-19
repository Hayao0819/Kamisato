package conf

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	"github.com/spf13/pflag"

	"github.com/Hayao0819/Kamisato/internal/client"
)

type MikoConfig struct {
	Debug             bool               `koanf:"debug"`
	Port              int                `koanf:"port"`
	Build             BuildServiceConfig `koanf:"build"`
	Builder           builder.HostConfig `koanf:"builder"`
	Executor          string             `koanf:"executor"` // deprecated: use builder.backend
	ArchBuildTemplate string             `koanf:"archbuild_template"`
	Concurrency       int                `koanf:"concurrency"`   // build workers (default 1)
	MaxRetries        int                `koanf:"max_retries"`   // retry attempts on failure (default 0)
	RetryBackoff      int                `koanf:"retry_backoff"` // seconds between retries (default 5)
	// MaxLogBytes caps a single job's in-memory log buffer (default 16 MiB).
	// Excess output is dropped after a truncation marker to bound memory.
	MaxLogBytes int `koanf:"max_log_bytes"`
	// MaxLogReaders caps concurrent SSE log readers per job (default 8) so a
	// single job cannot tie up unbounded streaming goroutines.
	MaxLogReaders int `koanf:"max_log_readers"`
	// MaxSize is the maximum package size in bytes. It uses the same name and
	// semantics as ayato.max_size; zero selects the shared secure default.
	MaxSize int `koanf:"max_size"`
	// Cache preserves the legacy Miko schema.
	Cache struct {
		Enabled        bool   `koanf:"enabled"`
		PacmanCacheDir string `koanf:"pacman_cache_dir"`
		CcacheDir      string `koanf:"ccache_dir"`
	} `koanf:"cache"`
	// APIKeys are accepted shared secrets for inbound requests (from ayato).
	// Deprecated: use auth.api_keys.
	APIKeys []string       `koanf:"api_keys"`
	Auth    MikoAuthConfig `koanf:"auth"`
	// AllowUnauthenticated explicitly permits an API with no shared key. This is
	// intended only for isolated development networks; the secure default is false.
	AllowUnauthenticated bool `koanf:"allow_unauthenticated"`
	// DataDir persists build jobs so they survive a restart. Empty disables
	// persistence (in-memory only).
	DataDir string `koanf:"data_dir"`
	// DockerHost preserves the legacy Miko schema.
	DockerHost string `koanf:"docker_host"`
	Ayato      struct {
		URL string `koanf:"url"`
		// APIKey is the Ayato service key.
		APIKey string `koanf:"api_key"`
		// Username and Password are deprecated Basic-auth settings.
		Username string `koanf:"username"`
		Password string `koanf:"password"`
	} `koanf:"ayato"`
	// Signing configures optional package signing. It is disabled unless Mode is
	// explicitly "local" or "remote"; KeyDir defaults to <data_dir>/keys only
	// for local signing.
	Signing struct {
		// Mode selects where signing runs: empty/"disabled" (default) does not sign;
		// "local" signs inline on the worker with its host key; "remote" offloads
		// to a dedicated signer service so the worker holds no private key.
		Mode   string `koanf:"mode"`
		KeyDir string `koanf:"key_dir"`
		Name   string `koanf:"name"`
		Email  string `koanf:"email"`
		// Remote is the signer service the worker calls in "remote" mode.
		Remote struct {
			// URL is the signer service base URL, e.g. http://miko-signer:8081.
			URL string `koanf:"url"`
			// APIKey authenticates the worker to the signer (sent as X-API-Key).
			// Prefer MIKO_SIGNING_REMOTE_API_KEY over the config file.
			APIKey string `koanf:"api_key"`
		} `koanf:"remote"`
	} `koanf:"signing"`
	// AURTrust gates which recursively-resolved AUR dependencies may be
	// auto-built when resolve_aur_deps is on.
	AURTrust AURTrustConfig `koanf:"aur_trust"`
	// NvCheck configures upstream version monitoring (nvchecker-style): each
	// entry watches a source for a newer version and rebuilds when one appears.
	NvCheck NvCheckConfig `koanf:"nvcheck"`
	// SonameRebuild, when true, walks a built package's shared objects after a
	// successful build and rebuilds its reverse-dependencies on a soname bump.
	SonameRebuild bool `koanf:"soname_rebuild"`
	// AURGitBase is the git base used to clone the PKGBUILD when a monitor or
	// soname bump rebuilds a package (default https://aur.archlinux.org).
	AURGitBase string `koanf:"aur_git_base"`
}

type MikoAuthConfig struct {
	APIKeys []MikoAPIKey `koanf:"api_keys"`
}

type MikoAPIKey struct {
	// Principal defaults to Name.
	Name      string   `koanf:"name"`
	Principal string   `koanf:"principal,omitempty"`
	Key       string   `koanf:"key"`
	Scopes    []string `koanf:"scopes"`
}

// NvCheckConfig configures upstream version monitoring.
type NvCheckConfig struct {
	// IntervalMin runs a periodic check every IntervalMin minutes; 0 disables the
	// ticker (the check can still be run on demand via CheckUpstreamVersions).
	IntervalMin int `koanf:"interval_min"`
	// Entries are the monitored packages.
	Entries []NvCheckEntry `koanf:"entries"`
}

// NvCheckEntry monitors one pkgbase against an upstream source and describes how
// to rebuild it when a newer version appears.
type NvCheckEntry struct {
	Pkgbase string `koanf:"pkgbase"`
	// Kind selects the source: "github", "github_tag", "pypi" or "http".
	Kind string `koanf:"kind"`
	// Repo is "owner/name" for the github/github_tag kinds.
	Repo string `koanf:"repo"`
	// Package is the project name for the pypi kind.
	Package string `koanf:"package"`
	// URL and Regex drive the "http" kind: fetch URL, extract the version from the
	// first capture group of Regex.
	URL   string `koanf:"url"`
	Regex string `koanf:"regex"`
	// Prefix is stripped from the matched version (e.g. "v" for "v1.2.3" tags).
	Prefix string `koanf:"prefix"`
	// BuildRepo and Arch target the rebuild; Git overrides the source clone URL
	// (default: <aur_git_base>/<pkgbase>.git).
	BuildRepo string `koanf:"build_repo"`
	Arch      string `koanf:"arch"`
	Git       string `koanf:"git"`
}

// AURTrustConfig is miko's build-time trust policy for AUR dependencies. It
// gates only the transitively-resolved dependencies of a submission, never the
// target the user explicitly asked to build: a malicious transitive dep would
// otherwise be built and published silently.
type AURTrustConfig struct {
	// TrustedMaintainers are AUR maintainer accounts whose packages may be
	// auto-built as dependencies (matched case-insensitively, as AUR does).
	TrustedMaintainers []string `koanf:"trusted_maintainers"`
	// TrustedPkgbases is an explicit pkgbase allowlist, auto-approved regardless
	// of maintainer.
	TrustedPkgbases []string `koanf:"trusted_pkgbases"`
	// AllowUntrusted, when true, builds an AUR dep that is neither whitelisted
	// nor from a trusted maintainer anyway (permissive; for closed/LAN setups).
	// The secure default (false) blocks such a dep with a needs-review error.
	AllowUntrusted bool `koanf:"allow_untrusted"`
}

func LoadMikoConfig(flags *pflag.FlagSet, configFile string) (*MikoConfig, error) {
	loadDotEnv()
	return loadTypedWithSourceTransforms[MikoConfig](
		commonConfigDirs(),
		configFileNames(configFile, "miko_config"),
		flags,
		"MIKO",
		(*MikoConfig).applyDefaults,
		migrateMikoBuilderSource,
	)
}

// applyDefaults fills unset fields and clamps out-of-range ones so every
// consumer reads a normalized config instead of re-deriving these at each use
// site. Run once, right after loading and before Validate.
func (c *MikoConfig) applyDefaults() {
	c.applyBuilderDefaults()
	if c.Port == 0 {
		c.Port = 8081
	}
	if c.Concurrency < 1 {
		c.Concurrency = 1
	}
	if c.MaxRetries < 0 {
		c.MaxRetries = 0
	}
	if c.RetryBackoff == 0 {
		c.RetryBackoff = 5
	}
	if c.MaxLogBytes == 0 {
		c.MaxLogBytes = 16 << 20
	}
	if c.MaxLogReaders == 0 {
		c.MaxLogReaders = 8
	}
	if c.AURGitBase == "" {
		c.AURGitBase = "https://aur.archlinux.org"
	}
}

// applyBuilderDefaults migrates legacy fields without overriding builder.*.
func (c *MikoConfig) applyBuilderDefaults() {
	if c.Builder.Backend == "" {
		if c.Executor != "" {
			c.Builder.Backend = builder.Kind(c.Executor)
		} else {
			c.Builder.Backend = builder.KindContainer
		}
	}
	if c.Builder.Timeout == 0 && c.Build.Timeout > 0 {
		c.Builder.Timeout = time.Duration(c.Build.Timeout) * time.Minute
	}
	if c.Builder.Docker.Image == "" {
		c.Builder.Docker.Image = c.Build.Image
	}
	if len(c.Builder.Repositories) == 0 {
		c.Builder.Repositories = append([]builder.PacmanRepository(nil), c.Build.ExtraRepos...)
	}
	if c.Builder.Docker.Host == "" {
		c.Builder.Docker.Host = c.DockerHost
	}
	if c.Cache.Enabled {
		if c.Builder.Docker.PacmanCacheDir == "" {
			c.Builder.Docker.PacmanCacheDir = c.Cache.PacmanCacheDir
		}
		if c.Builder.Docker.CcacheDir == "" {
			c.Builder.Docker.CcacheDir = c.Cache.CcacheDir
		}
		if c.Builder.Bwrap.PacmanCacheDir == "" {
			c.Builder.Bwrap.PacmanCacheDir = c.Cache.PacmanCacheDir
		}
	}
	if c.Builder.Devtools.ArchBuildTemplate == "" {
		if c.ArchBuildTemplate != "" {
			c.Builder.Devtools.ArchBuildTemplate = c.ArchBuildTemplate
		} else {
			c.Builder.Devtools.ArchBuildTemplate = "extra-%s-build"
		}
	}
	// Keep the legacy field consistent for callers that still inspect it.
	c.Executor = string(c.Builder.Backend)
}

// BuilderHostConfig also normalizes directly constructed MikoConfig values.
func (c *MikoConfig) BuilderHostConfig() builder.HostConfig {
	if c == nil {
		return builder.HostConfig{Backend: builder.KindContainer}
	}
	copyConfig := *c
	copyConfig.applyBuilderDefaults()
	return copyConfig.Builder
}

func (c *MikoConfig) Validate() error {
	if err := c.BuilderHostConfig().Validate(); err != nil {
		return fmt.Errorf("builder: %w", err)
	}
	if !slices.Contains([]string{"", "disabled", "local", "remote"}, c.Signing.Mode) {
		return fmt.Errorf("signing.mode: unknown value %q (want disabled, local or remote)", c.Signing.Mode)
	}
	if c.Signing.Mode == "remote" && c.Signing.Remote.URL == "" {
		return fmt.Errorf("signing.mode is remote but signing.remote.url is unset")
	}
	if c.Signing.Remote.URL != "" {
		if _, err := client.ParseBaseURL(c.Signing.Remote.URL); err != nil {
			return fmt.Errorf("signing.remote.url: %w", err)
		}
	}
	if c.Ayato.URL == "" && c.Ayato.APIKey != "" {
		return fmt.Errorf("ayato.api_key requires ayato.url")
	}
	if c.Ayato.URL != "" {
		if _, err := client.ParseBaseURL(c.Ayato.URL); err != nil {
			return fmt.Errorf("ayato.url: %w", err)
		}
		if c.Ayato.APIKey == "" {
			if c.Ayato.Username != "" || c.Ayato.Password != "" {
				return fmt.Errorf("ayato username/password Basic authentication is no longer supported; set ayato.api_key")
			}
			return fmt.Errorf("ayato.api_key is required when ayato.url is configured")
		}
	}
	knownScopes := map[string]bool{
		"*": true, "build:submit": true, "build:read": true,
		"build:cancel": true, "build:admin": true, "sign": true,
	}
	names := make(map[string]bool, len(c.Auth.APIKeys))
	keys := make(map[string]bool, len(c.Auth.APIKeys))
	for index, entry := range c.Auth.APIKeys {
		if entry.Name == "" || entry.Key == "" || len(entry.Scopes) == 0 {
			return fmt.Errorf("auth.api_keys[%d]: name, key, and scopes are required", index)
		}
		if entry.Principal == "" {
			entry.Principal = entry.Name
		}
		if strings.TrimSpace(entry.Principal) == "" {
			return fmt.Errorf("auth.api_keys[%d]: principal must not be blank", index)
		}
		if names[entry.Name] {
			return fmt.Errorf("auth.api_keys[%d]: duplicate name %q", index, entry.Name)
		}
		if keys[entry.Key] {
			return fmt.Errorf("auth.api_keys[%d]: duplicate key", index)
		}
		names[entry.Name] = true
		keys[entry.Key] = true
		for _, scope := range entry.Scopes {
			if !knownScopes[scope] {
				return fmt.Errorf("auth.api_keys[%d]: unknown scope %q", index, scope)
			}
		}
	}
	return nil
}
