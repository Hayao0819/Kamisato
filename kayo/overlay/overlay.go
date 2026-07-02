// Package overlay implements the kayo Backend: a set of trusted git overlays,
// each an AUR-identical repository (PKGBUILD + .SRCINFO at the root, split
// packages allowed), parsed statically into aurweb metadata. The PKGBUILD is
// never executed — only the committed .SRCINFO is read. Each overlay tracks its
// configured ref (or the remote default branch when unset); it is trusted by
// config, not cryptographically pinned, so pin to an immutable commit or tag if
// upstream must not move under you.
package overlay

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/gitcmd"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/kayo/pkgindex"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

// Registry is the live, swappable index over all overlays. It satisfies
// aurweb.Backend through the embedded index and is safe for concurrent use.
type Registry struct {
	*pkgindex.Index

	cacheDir string
	overlays []conf.OverlayConfig
}

// New builds an empty Registry. Call Sync before serving.
func New(cacheDir string, overlays []conf.OverlayConfig) *Registry {
	return &Registry{
		Index:    pkgindex.New(),
		cacheDir: cacheDir,
		overlays: overlays,
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

	r.Replace(index, bases)
	return nil
}

func fetchOverlay(ctx context.Context, dir string, o conf.OverlayConfig) error {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	exists := err == nil

	switch {
	case !exists:
		// Strict SSRF/loopback rejection is intentionally omitted: overlay URLs are
		// admin-config, and local/loopback overlays (e.g. a dumb-HTTP git server on
		// 127.0.0.1) are a supported deployment. ext:: RCE is blocked by gitcmd.
		return gitcmd.Clone(ctx, gitcmd.CloneOptions{URL: o.URL, Dir: dir, Ref: o.Ref})
	case o.Ref != "":
		if err := gitcmd.Run(ctx, dir, "fetch", "--quiet", "--tags", "--prune", "origin", o.Ref); err != nil {
			return err
		}
		// reset --hard FETCH_HEAD so a branch ref actually advances; a plain
		// checkout of an already-checked-out branch would freeze it at clone time.
		// FETCH_HEAD resolves the just-fetched ref be it a branch, tag, or commit.
		return gitcmd.Run(ctx, dir, "reset", "--hard", "--quiet", "FETCH_HEAD")
	default:
		return gitcmd.Run(ctx, dir, "pull", "--quiet", "--ff-only")
	}
}
