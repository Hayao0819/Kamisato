// Package build は ayaka のパッケージビルド操作をまとめる。
//
// もとは pkg/pacman/{pkg,repo} のメソッドだったが、builder(Docker SDK 依存)を
// ドメイン型に持ち込み、配布専用の ayato まで Docker を巻き込んでいた。利用者は
// ayaka だけなのでこちらへ移した。置き場所は暫定。
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
	"github.com/samber/lo"

	alpm "github.com/Hayao0819/dyalpm"

	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder/factory"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

// Target keeps signing outside backend configuration.
type Target struct {
	Config      builder.ResolvedConfig
	Arch        string
	SignKey     string
	InstallPkgs []string
	Output      io.Writer
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

	return nil
}

// selectPackages returns the packages in pkgs whose pkgbase or any sub-package
// name is in names; all of them when names is empty.
func selectPackages(pkgs []*pkg.SourcePackage, names []string) []*pkg.SourcePackage {
	if len(names) == 0 {
		return pkgs
	}
	var selected []*pkg.SourcePackage
	for _, name := range names {
		for _, p := range pkgs {
			if name == p.Base() || lo.Contains(p.Names(), name) {
				selected = append(selected, p)
				break
			}
		}
	}
	return selected
}

// filterByArch drops packages whose arch=() excludes arch ("any" matches all), so
// a mixed-arch source repo builds only what each PKGBUILD supports.
func filterByArch(pkgs []*pkg.SourcePackage, arch string) []*pkg.SourcePackage {
	var kept []*pkg.SourcePackage
	for _, p := range pkgs {
		if p.SupportsArch(arch) {
			kept = append(kept, p)
			continue
		}
		slog.Info("skipping package: arch not supported", "pkgbase", p.Base(), "arch", arch, "supports", p.Arches())
	}
	return kept
}

// diffPackages returns the source packages that are newer than (or missing
// from) the remote repo rr.
func diffPackages(src []*pkg.SourcePackage, rr *repo.RemoteRepo) []*pkg.SourcePackage {
	var toBuild []*pkg.SourcePackage
	for _, sp := range src {
		rp := rr.PkgByPkgBase(sp.Base())
		if rp == nil {
			slog.Warn("Package does not exist in remote repository", "pkgbase", sp.Base())
			toBuild = append(toBuild, sp)
			continue
		}
		if alpm.VerCmp(sp.Version(), rp.Version()) > 0 {
			slog.Debug("Local package is newer", "pkgbase", sp.Base(), "local", sp.Version(), "remote", rp.Version())
			toBuild = append(toBuild, sp)
		}
	}
	return toBuild
}

// Repo builds the named packages in r (all of them when none are named).
func Repo(r *repo.SourceRepo, t *Target, dest string, pkgs ...string) error {
	fulldstdir := path.Join(dest, t.Arch)
	var errs []error
	if err := os.MkdirAll(fulldstdir, 0o755); err != nil { //nolint:gosec // published pacman repo output dir is world-readable by design
		return err
	}

	targetPkgs := selectPackages(r.Pkgs, pkgs)
	if len(targetPkgs) == 0 {
		return fmt.Errorf("no packages found")
	}
	targetPkgs = filterByArch(targetPkgs, t.Arch)
	if len(targetPkgs) == 0 {
		slog.Info("No packages to build for arch", "arch", t.Arch)
		return nil
	}

	for _, p := range targetPkgs {
		slog.Info("building package", "pkg", p.Names())
		if err := Package(p, t, fulldstdir); err != nil {
			slog.Error("build package failed", "pkg", p.Names(), "err", err)
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Diff builds only the packages in s that are newer than (or missing from) the remote repo rr.
func Diff(s *repo.SourceRepo, t *Target, rr *repo.RemoteRepo, dest string, pkgs ...string) error {
	toBuild := diffPackages(s.Pkgs, rr)
	toBuild = selectPackages(toBuild, pkgs)
	toBuild = filterByArch(toBuild, t.Arch)

	if len(toBuild) == 0 {
		slog.Info("No packages to build")
		return nil
	}

	outDir := path.Join(dest, t.Arch)
	var errs []error
	for _, p := range toBuild {
		pkgbase := p.Base()
		slog.Debug("Starting package build", "pkgbase", pkgbase)
		if err := Package(p, t, outDir); err != nil {
			slog.Error("Package build failed", "pkgbase", pkgbase, "error", err)
			errs = append(errs, errors.WrapErr(err, "failed to build package: "+pkgbase))
			continue
		}
		slog.Debug("Package build completed", "pkgbase", pkgbase)
	}
	return errors.Join(errs...)
}
