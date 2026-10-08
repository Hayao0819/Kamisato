// Package build executes package builds: it drives a builder backend over the
// selected source packages in dependency order, then signs and (optionally)
// publishes each result. Planning what to build lives in service/plan.
package build

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"

	"github.com/Hayao0819/Kamisato/ayaka/source"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/pacman/builder"
	"github.com/Hayao0819/Kamisato/internal/pacman/pkg"
	"github.com/otiai10/copy"
)

// Target keeps signing and publishing outside backend configuration.
type Target struct {
	Backend      builder.Backend
	Arch         string
	Sign         func(string) error
	InstallPkgs  []string
	InstallNames []string
	Output       io.Writer
	// Publish, when non-nil, uploads a package's built files right after it is
	// built (and signed), so later builds in the same run can depend on it.
	Publish func(pkgPaths []string) error
}

// Package copies the SourcePackage to a temp directory, builds it, and signs it if needed.
func Package(ctx context.Context, p *pkg.SourcePackage, target *Target, dest string) error {
	if target == nil || target.Backend == nil {
		return fmt.Errorf("build backend is not configured")
	}
	tmpdir, err := os.MkdirTemp("", "ayaka-build-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpdir) }()
	slog.Info("tempdir", "dir", tmpdir)
	// The package dir itself may be a symlink (e.g. into a submodule); copy
	// resolves it or the tempdir would become a dangling link copy.
	srcdir, err := filepath.EvalSymlinks(p.Dir())
	if err != nil {
		return err
	}
	if err := copy.Copy(srcdir, tmpdir); err != nil {
		return err
	}
	result, err := target.Backend.Build(ctx, builder.Spec{
		SrcDir:       tmpdir,
		OutDir:       dest,
		Arch:         target.Arch,
		InstallPkgs:  target.InstallPkgs,
		InstallNames: target.InstallNames,
		LogWriter:    target.Output,
	})
	if err != nil {
		return errors.WrapErr(err, "failed to build package")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if result == nil {
		return fmt.Errorf("build backend returned no result")
	}

	if target.Sign != nil {
		for _, pkgPath := range result.Packages {
			if err := target.Sign(pkgPath); err != nil {
				return errors.WrapErr(err, "failed to sign file: "+pkgPath)
			}
		}
	}

	if target.Publish != nil {
		if err := target.Publish(result.Packages); err != nil {
			return errors.WrapErr(err, "failed to publish packages")
		}
	}

	return nil
}

// Repo builds the named packages in r (all of them when none are named).
func Repo(ctx context.Context, r *source.SourceRepo, t *Target, dest string, pkgs ...string) error {
	fulldstdir := path.Join(dest, t.Arch)
	var errs []error
	if err := os.MkdirAll(fulldstdir, 0o755); err != nil { //nolint:gosec // published pacman repo output dir is world-readable by design
		return err
	}

	targetPkgs := source.SelectPackages(r.Pkgs, pkgs)
	if len(targetPkgs) == 0 {
		return fmt.Errorf("no packages found")
	}
	targetPkgs = source.FilterByArch(targetPkgs, t.Arch)
	if len(targetPkgs) == 0 {
		slog.Info("No packages to build for arch", "arch", t.Arch)
		return nil
	}
	targetPkgs = source.OrderByDeps(targetPkgs, t.Arch)

	for _, p := range targetPkgs {
		if err := ctx.Err(); err != nil {
			return err
		}
		slog.Info("building package", "pkg", p.Names())
		if err := Package(ctx, p, t, fulldstdir); err != nil {
			slog.Error("build package failed", "pkg", p.Names(), "err", err)
			errs = append(errs, err)
			// When publishing, a later package may depend on this one's upload;
			// stop instead of building against a stale repo.
			if t.Publish != nil {
				break
			}
		}
	}
	return errors.Join(errs...)
}
