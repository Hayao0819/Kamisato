package source

import (
	"context"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Hayao0819/Kamisato/pkg/nvcheck"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// NvCheckEntry is one monitored package of a source repo: its parsed
// .nvchecker.toml, or ParseErr when the file is unreadable so the failure
// surfaces as a check error instead of a silent skip.
type NvCheckEntry struct {
	Pkgbase  string
	Current  string
	Spec     nvcheck.Spec
	ParseErr error
}

// CollectNvCheckEntries walks srcrepo's packages and returns one entry per
// package directory holding a .nvchecker.toml.
func CollectNvCheckEntries(srcrepo *repo.SourceRepo) []NvCheckEntry {
	var entries []NvCheckEntry
	for _, p := range srcrepo.Pkgs {
		path := filepath.Join(p.Dir(), ".nvchecker.toml")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		f, err := nvcheck.LoadFile(path)
		if err != nil {
			entries = append(entries, NvCheckEntry{Pkgbase: p.Base(), Current: p.Pkgver(), ParseErr: err})
			continue
		}
		entries = append(entries, NvCheckEntry{Pkgbase: p.Base(), Current: p.Pkgver(), Spec: f.Spec})
	}
	return entries
}

// RunNvCheck checks srcrepo's monitored packages against their upstreams,
// read-only. The local PKGBUILD pkgver is the current version (epoch and
// pkgrel excluded, as upstreams know neither).
func RunNvCheck(ctx context.Context, srcrepo *repo.SourceRepo, client *http.Client) []nvcheck.Result {
	entries := CollectNvCheckEntries(srcrepo)
	if len(entries) == 0 {
		return nil
	}

	current := make(map[string]string, len(entries))
	var checkerEntries []nvcheck.Entry
	var results []nvcheck.Result
	for _, e := range entries {
		if e.ParseErr != nil {
			results = append(results, nvcheck.Result{Pkgbase: e.Pkgbase, Current: e.Current, Err: e.ParseErr})
			continue
		}
		current[e.Pkgbase] = e.Current
		checkerEntries = append(checkerEntries, nvcheck.Entry{Pkgbase: e.Pkgbase, Source: e.Spec})
	}

	checker := nvcheck.NewChecker(checkerEntries, nvcheck.CheckerOptions{
		HTTPClient: client,
		CurrentVersion: func(_ context.Context, e nvcheck.Entry) (string, error) {
			return current[e.Pkgbase], nil
		},
	})
	return append(checker.Check(ctx), results...)
}
