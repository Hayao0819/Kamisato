// Package overlay implements the kayo Backend: a set of trusted git overlays,
// each an AUR-identical repository (PKGBUILD + .SRCINFO at the root, split
// packages allowed), parsed statically into aurweb metadata. The PKGBUILD is
// never executed — only the committed .SRCINFO is read — and refs are pinned, so
// an upstream change to an overlay cannot silently alter what kayo resolves.
package overlay

import (
	"cmp"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/gitcmd"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

// Registry is the live, swappable index over all overlays. It satisfies
// aurweb.Backend and is safe for concurrent use.
type Registry struct {
	cacheDir string
	overlays []conf.OverlayConfig

	mu    sync.RWMutex
	index map[string]aurweb.Pkg // pkgname -> record
	prio  map[string]int        // pkgname -> winning overlay priority
	bases map[string]string     // pkgbase -> git clone URL (redirect target)
	names []string              // sorted pkgnames, for suggest
}

// New builds an empty Registry. Call Sync before serving.
func New(cacheDir string, overlays []conf.OverlayConfig) *Registry {
	return &Registry{
		cacheDir: cacheDir,
		overlays: overlays,
		index:    map[string]aurweb.Pkg{},
		prio:     map[string]int{},
		bases:    map[string]string{},
	}
}

// Sync clones or updates every overlay, re-parses their .SRCINFO, and atomically
// swaps in a fresh index. A single failing overlay is logged and skipped.
func (r *Registry) Sync(ctx context.Context) error {
	if err := os.MkdirAll(r.cacheDir, 0o755); err != nil {
		return utils.WrapErr(err, "failed to create overlay cache dir")
	}

	index := map[string]aurweb.Pkg{}
	prio := map[string]int{}
	bases := map[string]string{}

	for _, o := range r.overlays {
		dir := filepath.Join(r.cacheDir, o.Name)
		if err := fetchOverlay(ctx, dir, o); err != nil {
			slog.Error("overlay sync failed; skipping", "overlay", o.Name, "error", err)
			continue
		}

		si, err := raiou.ParseSrcinfoFile(filepath.Join(dir, ".SRCINFO"))
		if err != nil {
			slog.Error("overlay .SRCINFO missing or invalid; skipping", "overlay", o.Name, "error", err)
			continue
		}

		ts := gitcmd.CommitUnix(ctx, dir)
		pkgs := aurweb.FromSrcinfo(si, aurweb.SrcinfoMeta{
			Maintainer:     o.Maintainer,
			Submitter:      o.Maintainer,
			FirstSubmitted: ts,
			LastModified:   ts,
		})
		if len(pkgs) == 0 {
			continue
		}

		bases[pkgs[0].PackageBase] = o.URL
		for _, p := range pkgs {
			if cur, ok := prio[p.Name]; ok && cur >= o.Priority {
				slog.Warn("overlay package shadowed by higher/equal priority overlay", "package", p.Name, "overlay", o.Name)
				continue
			}
			index[p.Name] = p
			prio[p.Name] = o.Priority
		}
		slog.Info("overlay synced", "overlay", o.Name, "packages", len(pkgs))
	}

	names := make([]string, 0, len(index))
	for n := range index {
		names = append(names, n)
	}
	slices.Sort(names)

	r.mu.Lock()
	r.index, r.prio, r.bases, r.names = index, prio, bases, names
	r.mu.Unlock()
	return nil
}

func (r *Registry) Info(_ context.Context, requested []string) ([]aurweb.Pkg, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []aurweb.Pkg
	for _, n := range requested {
		if p, ok := r.index[n]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r *Registry) Search(_ context.Context, by aurweb.By, arg string) ([]aurweb.Pkg, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []aurweb.Pkg
	for _, p := range r.index {
		if aurweb.Match(p, by, arg) {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(a, b aurweb.Pkg) int { return cmp.Compare(a.Name, b.Name) })
	return out, nil
}

func (r *Registry) Suggest(_ context.Context, arg string, pkgbase bool) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var pool []string
	if pkgbase {
		seen := map[string]bool{}
		for _, p := range r.index {
			if !seen[p.PackageBase] {
				seen[p.PackageBase] = true
				pool = append(pool, p.PackageBase)
			}
		}
		slices.Sort(pool)
	} else {
		pool = r.names
	}

	var out []string
	for _, n := range pool {
		if strings.HasPrefix(n, arg) {
			out = append(out, n)
			if len(out) >= 20 {
				break
			}
		}
	}
	return out, nil
}

func (r *Registry) All(_ context.Context) ([]aurweb.Pkg, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]aurweb.Pkg, 0, len(r.index))
	for _, p := range r.index {
		out = append(out, p)
	}
	return out, nil
}

func (r *Registry) SourceURL(_ context.Context, pkgbase string) (string, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if u, ok := r.bases[pkgbase]; ok {
		return u, true, nil
	}
	return "", false, nil
}

func fetchOverlay(ctx context.Context, dir string, o conf.OverlayConfig) error {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	exists := err == nil

	switch {
	case !exists:
		return gitcmd.Clone(ctx, gitcmd.CloneOptions{URL: o.URL, Dir: dir, Ref: o.Ref})
	case o.Ref != "":
		if err := gitcmd.Run(ctx, dir, "fetch", "--quiet", "--tags", "--prune", "origin"); err != nil {
			return err
		}
		return gitcmd.Run(ctx, dir, "checkout", "--quiet", o.Ref)
	default:
		return gitcmd.Run(ctx, dir, "pull", "--quiet", "--ff-only")
	}
}
