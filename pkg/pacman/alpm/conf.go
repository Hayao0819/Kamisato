package alpm

import (
	"os/exec"
	"strings"
)

// Fallback paths for when pacman-conf cannot be consulted. It ships with pacman
// and should always be present, but the resolvers degrade rather than fail.
const (
	FallbackHookDir  = "/etc/pacman.d/hooks"
	FallbackCacheDir = "/var/cache/pacman/pkg"
)

// ConfValues returns the values pacman-conf reports for a config option (e.g.
// "HookDir", "CacheDir"), one per line, with Include directives and built-in
// defaults already applied. pacmanConf overrides the config path when non-empty.
func ConfValues(pacmanConf, option string) ([]string, error) {
	args := make([]string, 0, 3)
	if pacmanConf != "" {
		args = append(args, "--config", pacmanConf)
	}
	args = append(args, option)
	out, err := exec.Command("pacman-conf", args...).Output()
	if err != nil {
		return nil, err
	}
	var vals []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			vals = append(vals, line)
		}
	}
	return vals, nil
}

// HookDir returns the directory to install an admin hook into: the first HookDir
// configured in pacman.conf (default /etc/pacman.d/hooks). Custom hooks belong
// here, not in libalpm's package-owned /usr/share/libalpm/hooks.
func HookDir(pacmanConf string) string {
	if vals, err := ConfValues(pacmanConf, "HookDir"); err == nil && len(vals) > 0 {
		return vals[0]
	}
	return FallbackHookDir
}

// CacheDirs returns the package cache directories from pacman.conf (default
// /var/cache/pacman/pkg), searched in the order pacman lists them.
func CacheDirs(pacmanConf string) []string {
	if vals, err := ConfValues(pacmanConf, "CacheDir"); err == nil && len(vals) > 0 {
		return vals
	}
	return []string{FallbackCacheDir}
}
