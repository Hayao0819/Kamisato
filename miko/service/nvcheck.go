package service

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/pacman/nvcheck"
	"github.com/Hayao0819/Kamisato/miko/domain"
)

// nvcheckInterval is how often the periodic monitor runs when enabled; a single
// loop is shared across worker goroutines.
func (s *Service) nvcheckInterval() time.Duration {
	return s.settings.VersionCheckInterval
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
	return checkVersions(ctx, s.settings.VersionCheckEntries, nvcheck.CheckerOptions{
		HTTPClient:     s.httpClient,
		CurrentVersion: publishedVersion(s.repositories, s.settings.AyatoURL != ""),
		Enqueuer:       enq,
		Logger:         slog.Default(),
	})
}

// CheckUpstreamVersions is the read-only version check. It needs no worker,
// queue, signer, or persistence store, so reporting never initializes them.
func CheckUpstreamVersions(ctx context.Context, entries []nvcheck.Entry, client *http.Client, repositories RepositoryDBReader) []nvcheck.Result {
	return checkVersions(ctx, entries, nvcheck.CheckerOptions{
		HTTPClient:     client,
		CurrentVersion: publishedVersion(repositories, repositories != nil),
		Logger:         slog.Default(),
	})
}

func checkVersions(ctx context.Context, entries []nvcheck.Entry, options nvcheck.CheckerOptions) []nvcheck.Result {
	if len(entries) == 0 {
		return nil
	}
	checker := nvcheck.NewChecker(entries, options)

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
	slog.Info("upstream version monitor started", "interval", interval, "entries", len(s.settings.VersionCheckEntries))
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
func publishedVersion(repositories RepositoryDBReader, configured bool) nvcheck.CurrentFunc {
	return func(ctx context.Context, entry nvcheck.Entry) (string, error) {
		if !configured || entry.Repo == "" || entry.Arch == "" {
			return "", nil
		}
		if repositories == nil {
			return "", errors.NewErr("Ayato repository reader is not configured")
		}
		rr, err := repositories.Database(ctx, entry.Repo, entry.Arch)
		if err != nil {
			return "", err
		}
		if p := rr.PkgByPkgBase(entry.Pkgbase); p != nil {
			return p.Version(), nil
		}
		return "", nil
	}
}
