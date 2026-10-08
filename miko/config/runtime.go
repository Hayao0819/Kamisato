package config

import (
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/pacman/nvcheck"
	"github.com/Hayao0819/Kamisato/miko/service"
)

func ServiceSettings(cfg *MikoConfig) service.Settings {
	if cfg == nil {
		return service.Settings{}
	}
	entries := make([]nvcheck.Entry, 0, len(cfg.NvCheck.Entries))
	for _, configured := range cfg.NvCheck.Entries {
		git := configured.Git
		if git == "" {
			git = strings.TrimRight(cfg.AURGitBase, "/") + "/" + configured.Pkgbase + ".git"
		}
		entries = append(entries, nvcheck.Entry{
			Pkgbase: configured.Pkgbase,
			Source: nvcheck.Spec{
				Kind:    configured.Kind,
				Repo:    configured.Repo,
				Package: configured.Package,
				URL:     configured.URL,
				Regex:   configured.Regex,
				Prefix:  configured.Prefix,
			},
			Repo: configured.BuildRepo,
			Arch: configured.Arch,
			Git:  git,
		})
	}
	return service.Settings{
		Builder:                   cfg.BuilderHostConfig(),
		ResolveAURDependencies:    cfg.Build.ResolveAURDeps,
		AyatoURL:                  cfg.Ayato.URL,
		Workers:                   cfg.Concurrency,
		Executor:                  cfg.Executor,
		DataDir:                   cfg.DataDir,
		VersionCheckInterval:      time.Duration(cfg.NvCheck.IntervalMin) * time.Minute,
		VersionCheckEntries:       entries,
		AURGitBase:                cfg.AURGitBase,
		SonameRebuild:             cfg.SonameRebuild,
		MaxPackageSize:            cfg.MaxSize,
		MaxRetries:                cfg.MaxRetries,
		RetryBackoff:              time.Duration(cfg.RetryBackoff) * time.Second,
		MaxLogBytes:               cfg.MaxLogBytes,
		TrustedAURMaintainers:     cfg.AURTrust.TrustedMaintainers,
		TrustedAURPackages:        cfg.AURTrust.TrustedPkgbases,
		AllowUntrustedAURPackages: cfg.AURTrust.AllowUntrusted,
	}
}
