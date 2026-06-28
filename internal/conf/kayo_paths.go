package conf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (c *KayoConfig) ListenAddr() string {
	bind := c.Bind
	if bind == "" {
		bind = "127.0.0.1"
	}
	return fmt.Sprintf("%s:%d", bind, c.Port)
}

func (c *KayoConfig) ResolvedCacheDir() string {
	if c.CacheDir != "" {
		return c.CacheDir
	}
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "kayo")
	}
	return filepath.Join(os.TempDir(), "kayo")
}

// ServedRoot is where kayo materializes pinned bare repos it serves directly
// (variant B), under the cache dir.
func (c *KayoConfig) ServedRoot() string {
	return filepath.Join(c.ResolvedCacheDir(), "served")
}

// AyatoPinStorePath is where TOFU-pinned ayato keys and the anti-rollback
// watermark live. It sits beside the trust store, not in the cache, so a cache
// wipe can't drop the pins (a downgrade vector).
func (c *KayoConfig) AyatoPinStorePath() string {
	return filepath.Join(filepath.Dir(c.ResolvedTrustStore()), "known_ayato.json")
}

func (c *KayoConfig) ResolvedTrustStore() string {
	if c.TrustStore != "" {
		return c.TrustStore
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "kayo", "trust.json")
	}
	return filepath.Join(os.TempDir(), "kayo-trust.json")
}

func (c *KayoConfig) AURGitBase() string {
	if c.Upstream.GitBase != "" {
		return strings.TrimRight(c.Upstream.GitBase, "/")
	}
	return "https://aur.archlinux.org"
}

func (c *KayoConfig) ResolvedEnforceMode() string {
	if c.EnforceMode == "enforce" {
		return "enforce"
	}
	return "warn"
}
