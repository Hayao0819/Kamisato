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

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/otiai10/copy"

	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder/factory"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

// Target keeps signing and publishing outside backend configuration.
type Target struct {
	Config      builder.ResolvedConfig
	Arch        string
	SignKey     string
	InstallPkgs []string
	Output      io.Writer
	// Publish, when non-nil, uploads a package's built files right after it is
	// built (and signed), so later builds in the same run can depend on it.
	Publish func(pkgPaths []string) error
}

// Package copies the SourcePackage to a temp directory, builds it, and signs it if needed.
func Package(p *pkg.SourcePackage, target *Target, dest string) error {
	tmpdir, err := os.MkdirTemp("", "ayaka-build-*")
	if err != nil {
		return err
	}
	slog.Info("tempdir", "dir", tmpdir)
	if err := copy.Copy(p.Dir(), tmpdir); err != nil {
		return err
	}
	// The output is moved to OutDir(=dest), so discard tmpdir holding the source copy.
	defer func() { _ = os.RemoveAll(tmpdir) }()

	backend, err := factory.New(target.Config)
	if err != nil {
		return errors.WrapErr(err, "failed to create build backend")
	}

	result, err := backend.Build(context.Background(), builder.Spec{
		SrcDir:      tmpdir,
		OutDir:      dest,
		Arch:        target.Arch,
		InstallPkgs: target.InstallPkgs,
		LogWriter:   target.Output,
	})
	if err != nil {
		return errors.WrapErr(err, "failed to build package")
	}

	if target.SignKey != "" {
		for _, pkgPath := range result.Packages {
			if err := sign.SignFile(target.SignKey, "", pkgPath); err != nil {
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
func Repo(r *repo.SourceRepo, t *Target, dest string, pkgs ...string) error {
	fulldstdir := path.Join(dest, t.Arch)
	var errs []error
	if err := os.MkdirAll(fulldstdir, 0o755); err != nil { //nolint:gosec // published pacman repo output dir is world-readable by design
		return err
	}

	targetPkgs := repo.SelectPackages(r.Pkgs, pkgs)
	if len(targetPkgs) == 0 {
		return fmt.Errorf("no packages found")
	}
	targetPkgs = repo.FilterByArch(targetPkgs, t.Arch)
	if len(targetPkgs) == 0 {
		slog.Info("No packages to build for arch", "arch", t.Arch)
		return nil
	}
	targetPkgs = repo.OrderByDeps(targetPkgs, t.Arch)

	for _, p := range targetPkgs {
		slog.Info("building package", "pkg", p.Names())
		if err := Package(p, t, fulldstdir); err != nil {
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

// Diff builds only the packages in s that are newer than (or missing from) the remote repo rr.
func Diff(s *repo.SourceRepo, t *Target, rr *repo.RemoteRepo, dest string, pkgs ...string) error {
	toBuild := repo.DiffPackages(s.Pkgs, rr)
	toBuild = repo.SelectPackages(toBuild, pkgs)
	toBuild = repo.FilterByArch(toBuild, t.Arch)

	if len(toBuild) == 0 {
		slog.Info("No packages to build")
		return nil
	}
	toBuild = repo.OrderByDeps(toBuild, t.Arch)

	outDir := path.Join(dest, t.Arch)
	var errs []error
	for _, p := range toBuild {
		pkgbase := p.Base()
		slog.Debug("Starting package build", "pkgbase", pkgbase)
		if err := Package(p, t, outDir); err != nil {
			slog.Error("Package build failed", "pkgbase", pkgbase, "error", err)
			errs = append(errs, errors.WrapErr(err, "failed to build package: "+pkgbase))
			if t.Publish != nil {
				break
			}
			continue
		}
		slog.Debug("Package build completed", "pkgbase", pkgbase)
	}
	return errors.Join(errs...)
}
