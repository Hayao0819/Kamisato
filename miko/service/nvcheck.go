package service

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/domain"
	"github.com/Hayao0819/Kamisato/miko/nvcheck"
)

// nvcheckInterval is how often the periodic monitor runs when enabled; a single
// loop is shared across worker goroutines.
func (s *Service) nvcheckInterval() time.Duration {
	return time.Duration(s.cfg.NvCheck.IntervalMin) * time.Minute
}

// CheckUpstreamVersions runs one version-check pass, enqueuing a
// ReasonVersionUpdate rebuild for each pkgbase whose upstream moved ahead of the
// published version. Best-effort: a per-entry failure is logged, not fatal.
func (s *Service) CheckUpstreamVersions(ctx context.Context) []nvcheck.Result {
	return s.runNvCheck(ctx, &versionUpdateEnqueuer{s: s})
}

// CheckUpstreamVersionsDryRun runs a check pass without triggering rebuilds, for
// the `miko nvcheck` CLI which runs out-of-process from the build worker.
func (s *Service) CheckUpstreamVersionsDryRun(ctx context.Context) []nvcheck.Result {
	return s.runNvCheck(ctx, nil)
}

func (s *Service) runNvCheck(ctx context.Context, enq nvcheck.Enqueuer) []nvcheck.Result {
	entries := nvcheckEntries(s.cfg)
	if len(entries) == 0 {
		return nil
	}
	checker := nvcheck.NewChecker(entries, nvcheck.CheckerOptions{
		HTTPClient:     s.httpClient,
		CurrentVersion: s.publishedVersion(),
		Enqueuer:       enq,
		Logger:         slog.Default(),
	})

	results := checker.Check(ctx)
	for _, r := range results {
		switch {
		case r.Err != nil:
			slog.Warn("nvcheck entry failed", "pkgbase", r.Pkgbase, "err", r.Err)
		case r.Enqueued:
			slog.Info("nvcheck queued rebuild", "pkgbase", r.Pkgbase, "current", r.Current, "latest", r.Latest)
		}
	}
	return results
}

// nvcheckLoop runs CheckUpstreamVersions on a ticker until ctx is cancelled. It
// is gated by nvcheck.interval_min and started once even with several workers.
func (s *Service) nvcheckLoop(ctx context.Context, interval time.Duration) {
	slog.Info("upstream version monitor started", "interval", interval, "entries", len(s.cfg.NvCheck.Entries))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.CheckUpstreamVersions(ctx)
		}
	}
}

// nvcheckEntries maps the config monitor entries to the nvcheck domain type,
// filling the clone URL default from aur_git_base.
func nvcheckEntries(cfg *conf.MikoConfig) []nvcheck.Entry {
	out := make([]nvcheck.Entry, 0, len(cfg.NvCheck.Entries))
	for _, e := range cfg.NvCheck.Entries {
		git := e.Git
		if git == "" {
			git = strings.TrimRight(cfg.AURGitBase, "/") + "/" + e.Pkgbase + ".git"
		}
		out = append(out, nvcheck.Entry{
			Pkgbase: e.Pkgbase,
			Source: nvcheck.Spec{
				Kind:    e.Kind,
				Repo:    e.Repo,
				Package: e.Package,
				URL:     e.URL,
				Regex:   e.Regex,
				Prefix:  e.Prefix,
			},
			Repo: e.BuildRepo,
			Arch: e.Arch,
			Git:  git,
		})
	}
	return out
}

// versionUpdateEnqueuer enqueues a monitored rebuild through the service, tagged
// ReasonVersionUpdate so its origin is visible in status and logs.
type versionUpdateEnqueuer struct{ s *Service }

func (e *versionUpdateEnqueuer) EnqueueVersionUpdate(entry nvcheck.Entry, newVersion string) error {
	req := &domain.BuildRequest{
		Repo: entry.Repo,
		Arch: entry.Arch,
		Git:  &domain.GitSource{URL: entry.Git},
	}
	id, err := e.s.submitWithReason(req, domain.ReasonVersionUpdate)
	if err != nil {
		return err
	}
	slog.Info("submitted version-update rebuild", "id", id, "pkgbase", entry.Pkgbase, "version", newVersion)
	return nil
}

// publishedVersion returns a CurrentFunc reading an entry's published version from
// ayato's repo DB. A missing repository or package resolves to an empty version,
// which the checker treats as out-of-date so the first pass baselines. Transport
// and malformed-database failures remain visible instead of looking outdated.
func (s *Service) publishedVersion() nvcheck.CurrentFunc {
	return func(ctx context.Context, entry nvcheck.Entry) (string, error) {
		if s.cfg.Ayato.URL == "" || entry.Repo == "" || entry.Arch == "" {
			return "", nil
		}
		rr, err := s.repositoryDB(ctx, entry.Repo, entry.Arch)
		if err != nil {
			return "", err
		}
		if p := rr.PkgByPkgBase(entry.Pkgbase); p != nil {
			return p.Version(), nil
		}
		return "", nil
	}
}
