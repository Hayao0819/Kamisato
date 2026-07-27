package source

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/gitcmd"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

const aurHost = "aur.archlinux.org"

// MirrorPackages returns the packages whose directory is its own git checkout
// (a submodule or plain clone tracking an origin).
func MirrorPackages(src *repo.SourceRepo) []*pkg.SourcePackage {
	var mirrors []*pkg.SourcePackage
	for _, p := range src.Pkgs {
		if isGitCheckout(p.Dir()) {
			mirrors = append(mirrors, p)
		}
	}
	return mirrors
}

// IsAURMirror reports whether the package directory is a checkout tracking
// aur.archlinux.org.
func IsAURMirror(dir string) bool {
	if !isGitCheckout(dir) {
		return false
	}
	origin, err := gitcmd.OriginURL(dir)
	if err != nil {
		return false
	}
	return isAURURL(origin)
}

// isGitCheckout matches both plain clones (.git directory) and submodules
// (.git gitdir pointer file).
func isGitCheckout(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func isAURURL(raw string) bool {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Hostname() == aurHost
	}
	// scp-style remote: [user@]host:path
	if _, rest, ok := strings.Cut(raw, "@"); ok {
		raw = rest
	}
	host, _, ok := strings.Cut(raw, ":")
	return ok && host == aurHost
}

// PullPackages hard-syncs each named mirror package (every mirror when names is
// empty) to its origin HEAD and returns the reloaded packages. It works on
// detached-HEAD checkouts (CI submodules) and refuses a dirty worktree unless
// force discards it. Per-package failures are collected so one bad mirror does
// not stop the rest.
func PullPackages(ctx context.Context, src *repo.SourceRepo, names []string, force bool) ([]*pkg.SourcePackage, error) {
	targets, err := pullTargets(src, names)
	if err != nil {
		return nil, err
	}

	var pulled []*pkg.SourcePackage
	var failures []string
	for _, p := range targets {
		reloaded, err := pullOne(ctx, p, force)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		pulled = append(pulled, reloaded)
	}
	if len(failures) > 0 {
		return pulled, errors.NewErr("one or more pulls failed:\n" + strings.Join(failures, "\n"))
	}
	return pulled, nil
}

func pullTargets(src *repo.SourceRepo, names []string) ([]*pkg.SourcePackage, error) {
	if len(names) == 0 {
		return MirrorPackages(src), nil
	}
	var targets []*pkg.SourcePackage
	for _, name := range names {
		p := findPackage(src.Pkgs, name)
		if p == nil {
			return nil, errors.NewErr("package not found: " + name)
		}
		if !isGitCheckout(p.Dir()) {
			return nil, errors.NewErr("package " + name + " is not a git checkout; nothing to pull")
		}
		targets = append(targets, p)
	}
	return targets, nil
}

func pullOne(ctx context.Context, p *pkg.SourcePackage, force bool) (*pkg.SourcePackage, error) {
	dir := p.Dir()
	clean, err := gitcmd.IsClean(dir)
	if err != nil {
		return nil, errors.WrapErr(err, p.Base())
	}
	if !clean && !force {
		return nil, errors.NewErr(p.Base() + " has local changes; use --force to discard them")
	}
	branch, err := gitcmd.OriginHead(ctx, dir)
	if err != nil {
		return nil, errors.WrapErr(err, p.Base())
	}
	if err := gitcmd.SyncHard(ctx, dir, branch); err != nil {
		return nil, errors.WrapErr(err, p.Base())
	}
	reloaded, err := pkg.OpenSourcePackage(dir)
	if err != nil {
		return nil, errors.WrapErr(err, "failed to reload "+p.Base()+" after pull")
	}
	return reloaded, nil
}
