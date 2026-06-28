package conf

import (
	"log/slog"

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
}

func LoadMikoConfig(flags *pflag.FlagSet, configFile string) (*MikoConfig, error) {
	if err := LoadEnv(); err != nil {
		slog.Error("Failed to load env", "error", err)
	}

	dirs := commonConfigDirs()
	files := []string{}
	if configFile != "" {
		files = append(files, configFile)
	} else {
		files = []string{"miko_config.json", "miko_config.toml", "miko_config.yaml"}
	}

	return loadConfig[MikoConfig](
		dirs,
		files,
		flags,
		"MIKO",
	)
}
