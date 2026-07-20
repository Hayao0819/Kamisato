package pacman

import (
	"fmt"
	"strings"

	pacmanconf "github.com/Morganamilo/go-pacmanconf"
)

const (
	FallbackHookDir  = "/etc/pacman.d/hooks"
	FallbackCacheDir = "/var/cache/pacman/pkg"
)

func loadConfig(path string) (*pacmanconf.Config, error) {
	var (
		config *pacmanconf.Config
		stderr string
		err    error
	)
	if path == "" {
		config, stderr, err = pacmanconf.PacmanConf()
	} else {
		config, stderr, err = pacmanconf.ParseFile(path)
	}
	if err == nil {
		return config, nil
	}
	if stderr = strings.TrimSpace(stderr); stderr != "" {
		return nil, fmt.Errorf("pacman-conf: %w: %s", err, stderr)
	}
	return nil, fmt.Errorf("pacman-conf: %w", err)
}

func HookDir(configPath string) string {
	if config, err := loadConfig(configPath); err == nil && len(config.HookDir) > 0 {
		return config.HookDir[0]
	}
	return FallbackHookDir
}

func CacheDirs(configPath string) []string {
	if config, err := loadConfig(configPath); err == nil && len(config.CacheDir) > 0 {
		return config.CacheDir
	}
	return []string{FallbackCacheDir}
}
