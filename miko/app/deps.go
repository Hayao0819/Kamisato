package app

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/ayatoapi"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/nvcheck"
	mikoconfig "github.com/Hayao0819/Kamisato/miko/config"
	"github.com/Hayao0819/Kamisato/miko/service"
)

func ServiceSettings(cfg *mikoconfig.MikoConfig) service.Settings {
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
		AURRPCURL:                 cfg.Build.AURRPCURL,
		AyatoURL:                  cfg.Ayato.URL,
		AyatoAPIKey:               cfg.Ayato.APIKey,
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

func CheckUpstreamVersions(ctx context.Context, cfg *mikoconfig.MikoConfig) ([]nvcheck.Result, error) {
	options, err := ServiceDependencies(cfg)
	if err != nil {
		return nil, err
	}
	return service.New(ServiceSettings(cfg), options...).CheckUpstreamVersionsDryRun(ctx), nil
}

func ServiceDependencies(cfg *mikoconfig.MikoConfig) ([]service.ServiceOption, error) {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	options := []service.ServiceOption{service.WithOutboundHTTPClient(httpClient)}
	if cfg.Ayato.URL == "" {
		return options, nil
	}

	repositories, err := ayatoapi.NewRepository(
		cfg.Ayato.URL,
		ayatoapi.WithHTTPClient(httpClient),
	)
	if err != nil {
		return nil, errors.WrapErr(err, "configure Ayato repository reader")
	}
	return append(options, service.WithRepositoryDBReader(repositories)), nil
}
