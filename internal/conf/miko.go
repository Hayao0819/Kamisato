package conf

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/confloader"
	"github.com/spf13/pflag"
)

type MikoConfig struct {
	Debug    bool        `koanf:"debug"`
	Port     int         `koanf:"port"`
	Build    BuildConfig `koanf:"build"`
	Executor string      `koanf:"executor"` // build backend kind: "container" | "chroot" (default: "container")
	// ArchBuildTemplate is the devtools wrapper name template for the chroot
	// executor, formatted with the target CARCH (default "extra-%s-build").
	ArchBuildTemplate string `koanf:"archbuild_template"`
	Concurrency       int    `koanf:"concurrency"`   // build workers (default 1)
	MaxRetries        int    `koanf:"max_retries"`   // retry attempts on failure (default 0)
	RetryBackoff      int    `koanf:"retry_backoff"` // seconds between retries (default 5)
	// MaxLogBytes caps a single job's in-memory log buffer (default 16 MiB).
	// Excess output is dropped after a truncation marker to bound memory.
	MaxLogBytes int `koanf:"max_log_bytes"`
	// MaxLogReaders caps concurrent SSE log readers per job (default 8) so a
	// single job cannot tie up unbounded streaming goroutines.
	MaxLogReaders int `koanf:"max_log_readers"`
	Cache         struct {
		Enabled        bool   `koanf:"enabled"`
		PacmanCacheDir string `koanf:"pacman_cache_dir"`
		CcacheDir      string `koanf:"ccache_dir"`
	} `koanf:"cache"`
	// APIKeys are accepted shared secrets for inbound requests (from ayato).
	// Empty means no key required (trust the closed network only).
	APIKeys []string `koanf:"api_keys"`
	// DataDir persists build jobs so they survive a restart. Empty disables
	// persistence (in-memory only).
	DataDir string `koanf:"data_dir"`
	// DockerHost overrides the Docker daemon for the container executor. Empty
	// falls back to DOCKER_HOST, then the active docker context, then the
	// default socket.
	DockerHost string `koanf:"docker_host"`
	Ayato      struct {
		URL      string `koanf:"url"`
		Username string `koanf:"username"`
		Password string `koanf:"password"`
	} `koanf:"ayato"`
	// Signing configures the worker host signing key. KeyDir defaults to
	// <data_dir>/keys; empty (with no data_dir) disables host signing.
	Signing struct {
		KeyDir string `koanf:"key_dir"`
		Name   string `koanf:"name"`
		Email  string `koanf:"email"`
	} `koanf:"signing"`
	// AURTrust gates which recursively-resolved AUR dependencies may be
	// auto-built when resolve_aur_deps is on.
	AURTrust AURTrustConfig `koanf:"aur_trust"`
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

// AURTrustDecision is the outcome of evaluating the trust policy for one dep.
type AURTrustDecision int

const (
	// AURTrustBlocked means the dep is neither trusted nor allowed: fail closed.
	AURTrustBlocked AURTrustDecision = iota
	// AURTrustByPkgbase means the pkgbase is on the explicit allowlist.
	AURTrustByPkgbase
	// AURTrustByMaintainer means the maintainer is trusted.
	AURTrustByMaintainer
	// AURTrustUntrusted means the dep is untrusted but allowed via allow_untrusted.
	AURTrustUntrusted
)

// Decide classifies an AUR dependency by pkgbase and maintainer. An empty
// maintainer (orphaned package) is never trusted on its own — orphans are a
// known takeover vector — so it can pass only via the pkgbase allowlist or
// allow_untrusted.
func (c AURTrustConfig) Decide(pkgbase, maintainer string) AURTrustDecision {
	if slices.Contains(c.TrustedPkgbases, pkgbase) {
		return AURTrustByPkgbase
	}
	if maintainer != "" && slices.ContainsFunc(c.TrustedMaintainers, func(m string) bool {
		return strings.EqualFold(m, maintainer)
	}) {
		return AURTrustByMaintainer
	}
	if c.AllowUntrusted {
		return AURTrustUntrusted
	}
	return AURTrustBlocked
}

func LoadMikoConfig(flags *pflag.FlagSet, configFile string) (*MikoConfig, error) {
	loadDotEnv()
	return confloader.LoadTyped[MikoConfig](
		commonConfigDirs(),
		configFileNames(configFile, "miko_config"),
		flags,
		"MIKO",
		(*MikoConfig).applyDefaults,
	)
}

// applyDefaults fills unset fields and clamps out-of-range ones so every
// consumer reads a normalized config instead of re-deriving these at each use
// site. Run once, right after loading and before Validate.
func (c *MikoConfig) applyDefaults() {
	if c.Executor == "" {
		c.Executor = "container"
	}
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
}

// Validate rejects an unknown build executor so a typo fails loudly instead of
// silently falling back to the default backend.
func (c *MikoConfig) Validate() error {
	if !slices.Contains([]string{"", "container", "chroot", "bwrap"}, c.Executor) {
		return fmt.Errorf("executor: unknown value %q (want container, chroot or bwrap)", c.Executor)
	}
	return nil
}
