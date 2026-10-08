package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/vcs/git"
	kayoconfig "github.com/Hayao0819/Kamisato/kayo/config"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

// Resolved is an audit target reduced to what the trust model needs: source,
// pkgbase, the maintainer ACCOUNT that owns it, and the commit.
type Resolved struct {
	Dir        string
	Source     string
	Pkgbase    string
	Maintainer string
	Commit     string
}

// Resolve turns a target (a directory, a git URL, or a package name) into a
// checked-out dir plus its provenance. cleanup must be called.
func Resolve(ctx context.Context, cfg *kayoconfig.KayoConfig, target, ref string) (Resolved, func(), error) {
	cleanup := func() {}

	r := Resolved{}
	if st, statErr := os.Stat(target); statErr == nil && st.IsDir() {
		r.Dir, r.Source = target, "local"
	} else {
		clone := git.CloneOptions{URL: target, Ref: ref}
		r.Source = target
		if !strings.Contains(target, "://") && !strings.HasSuffix(target, ".git") {
			var err error
			clone, r, err = resolvePackageSource(ctx, cfg, target)
			if err != nil {
				return Resolved{}, cleanup, err
			}
			if ref != "" {
				clone.Ref = ref
			}
		}
		var err error
		// Explicit targets and configured overlays support local/loopback
		// repositories. Catalog-selected URLs use the strict transport instead.
		r.Dir, cleanup, err = git.CloneTemp(ctx, "kayo-audit-*", clone)
		if err != nil {
			return Resolved{}, func() {}, err
		}
	}

	r.Commit, _ = git.HeadCommit(ctx, r.Dir)
	if r.Pkgbase == "" {
		r.Pkgbase = readPkgbase(r.Dir, target)
	}
	if r.Source == "aur" {
		r.Maintainer, r.Pkgbase = aurMeta(ctx, cfg, target, r.Pkgbase)
	}
	return r, cleanup, nil
}

// resolveForReview preserves working-tree audits, but local approvals inspect
// an owned checkout of a fixed commit. A clean check alone would race with an
// edit or HEAD change while the static and advisory passes run.
func resolveForReview(ctx context.Context, cfg *kayoconfig.KayoConfig, target, ref string, approve bool) (Resolved, func(), error) {
	resolved, cleanup, err := Resolve(ctx, cfg, target, ref)
	// A plain directory must retain the audit and RequirePinnedCommit error
	// path. Source names such as "local" may also belong to an Ayato instance.
	if err != nil || !approve || resolved.Source != "local" || resolved.Dir != target || resolved.Commit == "" {
		return resolved, cleanup, err
	}
	if ref == "" {
		clean, err := git.IsClean(resolved.Dir)
		if err != nil {
			cleanup()
			return Resolved{}, func() {}, errors.WrapErr(err, "check local approval target")
		}
		if !clean {
			cleanup()
			return Resolved{}, func() {}, errors.NewErr("local repository has uncommitted changes; commit them or select --ref before approval")
		}
		ref = resolved.Commit
	}
	dir, snapshotCleanup, err := git.CloneTemp(ctx, "kayo-review-*", git.CloneOptions{URL: resolved.Dir, Ref: ref})
	cleanup()
	if err != nil {
		return Resolved{}, func() {}, errors.WrapErr(err, "snapshot local approval target")
	}
	commit, err := git.HeadCommit(ctx, dir)
	if err != nil {
		snapshotCleanup()
		return Resolved{}, func() {}, errors.WrapErr(err, "resolve local approval commit")
	}
	resolved.Dir, resolved.Commit = dir, commit
	resolved.Pkgbase = readPkgbase(dir, target)
	return resolved, snapshotCleanup, nil
}

// Named targets follow the same source priority and maintainer identity as the
// daemon and verification hook, including split packages. Do not fall through to
// AUR when a managed record has no checkout: that would approve another source.
func resolvePackageSource(ctx context.Context, cfg *kayoconfig.KayoConfig, name string) (git.CloneOptions, Resolved, error) {
	if len(cfg.Overlays) > 0 || len(cfg.Ayato) > 0 {
		composite, overlays, err := BuildComposite(ctx, cfg)
		if err != nil {
			return git.CloneOptions{}, Resolved{}, err
		}
		if pkg, source, _, ok := composite.Resolve(ctx, name); ok {
			resolved := Resolved{Source: source, Pkgbase: pkg.PackageBase, Maintainer: pkg.Maintainer}
			if source == "overlay" && overlays != nil {
				dir := overlays.SourceDirs()[pkg.PackageBase]
				commit, err := git.HeadCommit(ctx, dir)
				if err != nil {
					return git.CloneOptions{}, Resolved{}, errors.WrapErr(err, "resolve overlay checkout for "+name)
				}
				return git.CloneOptions{URL: dir, Ref: commit}, resolved, nil
			}
			url, found, err := composite.SourceURLFor(ctx, source, pkg.PackageBase)
			if err != nil {
				return git.CloneOptions{}, Resolved{}, errors.WrapErr(err, "resolve checkout for "+name)
			}
			if !found || url == "" {
				return git.CloneOptions{}, Resolved{}, errors.NewErrf("%s from %s has no source repository", name, source)
			}
			// A catalog authenticates its publisher, not permission to read a
			// local repository or contact arbitrary private-network services.
			return git.CloneOptions{URL: url, Strict: true}, resolved, nil
		}
	}
	return git.CloneOptions{URL: cfg.AURGitBase() + "/" + name + ".git"}, Resolved{Source: "aur"}, nil
}

// RequirePinnedCommit fails when the target has no commit to pin, which happens
// for a local directory that is not a git repo (HeadCommit left Commit empty).
// Pinning paths call this so the user gets a clear message here instead of the
// opaque failure deferred to gitserve.Materialize. The audit path does not call
// it: auditing a plain directory with no commit is legitimate.
func (r Resolved) RequirePinnedCommit() error {
	if r.Commit == "" {
		return errors.NewErr("target is not a git repository; cannot pin a reviewed commit")
	}
	return nil
}

func readPkgbase(dir, fallback string) string {
	if si, err := raiou.ParseSrcinfoFile(filepath.Join(dir, ".SRCINFO")); err == nil && si.PkgBase != "" {
		return si.PkgBase
	}
	return filepath.Base(fallback)
}

// aurMeta best-effort fetches the maintainer account (and authoritative pkgbase)
// for an AUR package from the upstream RPC. The maintainer account, not any git
// email, is the trust anchor.
func aurMeta(ctx context.Context, cfg *kayoconfig.KayoConfig, name, pkgbase string) (maintainer, base string) {
	up := aurweb.NewAURUpstream(cfg.Upstream.RPCURL, aurweb.WithGitBase(cfg.AURGitBase()))
	pkgs, err := up.Info(ctx, []string{name})
	if err != nil || len(pkgs) == 0 {
		return "", pkgbase
	}
	base = pkgbase
	if pkgs[0].PackageBase != "" {
		base = pkgs[0].PackageBase
	}
	return pkgs[0].Maintainer, base
}
