package source

import (
	"context"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Hayao0819/Kamisato/pkg/nvcheck"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// CheckMethod is how an outdated package gets updated: rewriting its PKGBUILD
// (nvbump) or advancing its mirror checkout to origin (pull).
type CheckMethod string

const (
	MethodNvBump CheckMethod = "nvbump"
	MethodPull   CheckMethod = "pull"
)

// NvCheckEntry is one monitored package of a source repo: its parsed
// .nvchecker.toml or AUR-mirror watch, or ParseErr when the config is
// unreadable so the failure surfaces as a check error instead of a silent skip.
type NvCheckEntry struct {
	Pkgbase  string
	Current  string
	Spec     nvcheck.Spec
	Method   CheckMethod
	ParseErr error
}

// CheckResult pairs an upstream check outcome with the update method the CI
// pipeline must apply.
type CheckResult struct {
	nvcheck.Result
	Method CheckMethod
}

// CollectNvCheckEntries walks srcrepo's packages. An AUR-origin checkout is
// watched against the AUR RPC version (full version, pkgrel included — a pull
// helps exactly when the AUR maintainer pushed); its bundled .nvchecker.toml is
// the AUR maintainer's own upstream watch and is ignored. Other packages are
// watched per their .nvchecker.toml, against the bare pkgver.
func CollectNvCheckEntries(srcrepo *repo.SourceRepo) []NvCheckEntry {
	var entries []NvCheckEntry
	for _, p := range srcrepo.Pkgs {
		if IsAURMirror(p.Dir()) {
			entries = append(entries, NvCheckEntry{
				Pkgbase: p.Base(),
				Current: p.Version(),
				Spec:    nvcheck.Spec{Kind: "aur", Package: p.Base()},
				Method:  MethodPull,
			})
			continue
		}
		path := filepath.Join(p.Dir(), ".nvchecker.toml")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		entry := NvCheckEntry{Pkgbase: p.Base(), Current: p.Pkgver(), Method: MethodNvBump}
		f, err := nvcheck.LoadFile(path)
		if err != nil {
			entry.ParseErr = err
		} else {
			entry.Spec = f.Spec
		}
		entries = append(entries, entry)
	}
	return entries
}

// RunNvCheck checks srcrepo's monitored packages against their upstreams,
// read-only.
func RunNvCheck(ctx context.Context, srcrepo *repo.SourceRepo, client *http.Client) []CheckResult {
	entries := CollectNvCheckEntries(srcrepo)
	if len(entries) == 0 {
		return nil
	}

	current := make(map[string]string, len(entries))
	methods := make(map[string]CheckMethod, len(entries))
	var checkerEntries []nvcheck.Entry
	var results []CheckResult
	for _, e := range entries {
		if e.ParseErr != nil {
			results = append(results, CheckResult{
				Result: nvcheck.Result{Pkgbase: e.Pkgbase, Current: e.Current, Err: e.ParseErr},
				Method: e.Method,
			})
			continue
		}
		current[e.Pkgbase] = e.Current
		methods[e.Pkgbase] = e.Method
		checkerEntries = append(checkerEntries, nvcheck.Entry{Pkgbase: e.Pkgbase, Source: e.Spec})
	}

	checker := nvcheck.NewChecker(checkerEntries, nvcheck.CheckerOptions{
		HTTPClient: client,
		CurrentVersion: func(_ context.Context, e nvcheck.Entry) (string, error) {
			return current[e.Pkgbase], nil
		},
	})
	for _, r := range checker.Check(ctx) {
		results = append(results, CheckResult{Result: r, Method: methods[r.Pkgbase]})
	}
	return results
}
