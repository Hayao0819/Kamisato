package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/domain"
	"github.com/Hayao0819/Kamisato/miko/nvcheck"
	"github.com/Hayao0819/Kamisato/pkg/httpx"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
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
	client := httpx.Default()
	checker := nvcheck.NewChecker(entries, client, s.publishedVersion(client), enq, slog.Default())

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
// ayato's repo DB. A missing entry or unreachable repo resolves to an empty
// version, which the checker treats as out-of-date so the first pass baselines.
func (s *Service) publishedVersion(client *http.Client) nvcheck.CurrentFunc {
	return func(ctx context.Context, entry nvcheck.Entry) (string, error) {
		if s.cfg.Ayato.URL == "" || entry.Repo == "" || entry.Arch == "" {
			return "", nil
		}
		base := strings.TrimRight(s.cfg.Ayato.URL, "/") + "/repo/" + entry.Repo + "/" + entry.Arch
		dbURL := base + "/" + entry.Repo + ".db"

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, dbURL, nil)
		if err != nil {
			return "", err
		}
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode == http.StatusNotFound {
			return "", nil // repo not created yet
		}
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("nvcheck: fetch %s: status %d", dbURL, resp.StatusCode)
		}
		rr, err := repo.RemoteRepoFromDB(entry.Repo, resp.Body)
		if err != nil {
			return "", err
		}
		if p := rr.PkgByPkgBase(entry.Pkgbase); p != nil {
			return p.Version(), nil
		}
		return "", nil
	}
}
