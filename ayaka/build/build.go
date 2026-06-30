// Package build は ayaka のパッケージビルド操作をまとめる。
//
// もとは pkg/pacman/{pkg,repo} のメソッドだったが、builder(Docker SDK 依存)を
// ドメイン型に持ち込み、配布専用の ayato まで Docker を巻き込んでいた。利用者は
// ayaka だけなのでこちらへ移した。置き場所は暫定。
package build

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/pacman/alpm"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	"github.com/Hayao0819/Kamisato/pkg/pacman/gpg"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/samber/lo"
)

// Package copies the SourcePackage to a temp directory, builds it, and signs it if needed.
func Package(p *pkg.SourcePackage, target *builder.Target, dest string) error {
	tmpdir, err := os.MkdirTemp("", "ayaka-build-*")
	if err != nil {
		return err
	}
	slog.Info("tempdir", "dir", tmpdir)
	if err := utils.CopyDir(p.Dir(), tmpdir); err != nil {
		return err
	}
	// The output is moved to OutDir(=dest), so discard tmpdir holding the source copy.
	defer func() { _ = os.RemoveAll(tmpdir) }()

	kind := target.Executor
	if kind == "" {
		kind = builder.KindChroot
	}
	backend, err := builder.New(kind, builder.Options{})
	if err != nil {
		return utils.WrapErr(err, "failed to create build backend")
	}

	result, err := backend.Build(context.Background(), builder.Spec{
		SrcDir:      tmpdir,
		OutDir:      dest,
		Arch:        target.Arch,
		ArchBuild:   target.ArchBuild,
		InstallPkgs: target.InstallPkgs,
		LogWriter:   target.Output,
	})
	if err != nil {
		return utils.WrapErr(err, "failed to build package")
	}

	if target.SignKey != "" {
		for _, pkgPath := range result.Packages {
			if err := gpg.SignFile(target.SignKey, "", pkgPath); err != nil {
				return utils.WrapErr(err, "failed to sign file: "+pkgPath)
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

// diffPackages returns the source packages that are newer than (or missing
// from) the remote repo rr.
func diffPackages(src []*pkg.SourcePackage, rr *repo.RemoteRepo) ([]*pkg.SourcePackage, error) {
	var shoubuild []*pkg.SourcePackage
	for _, sp := range src {
		rp := rr.PkgByPkgBase(sp.Base())
		if rp == nil {
			slog.Warn("Package does not exist in remote repository", "pkgbase", sp.Base())
			shoubuild = append(shoubuild, sp)
			continue
		}
		cmp, err := alpm.VerCmp(sp.Version(), rp.Version())
		if err != nil {
			slog.Error("Failed to compare versions", "pkgbase", sp.Base(), "error", err)
			return nil, utils.WrapErr(err, "failed to compare package versions")
		}
		if cmp > 0 {
			slog.Debug("Local package is newer", "pkgbase", sp.Base(), "local", sp.Version(), "remote", rp.Version())
			shoubuild = append(shoubuild, sp)
		}
	}
	return shoubuild, nil
}

// Repo builds the named packages in r (all of them when none are named).
func Repo(r *repo.SourceRepo, t *builder.Target, dest string, pkgs ...string) error {
	fulldstdir := path.Join(dest, t.Arch)
	var errs []error
	if err := os.MkdirAll(fulldstdir, 0755); err != nil {
		return err
	}

	targetPkgs := selectPackages(r.Pkgs, pkgs)
	if len(targetPkgs) == 0 {
		return fmt.Errorf("no packages found")
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
func Diff(s *repo.SourceRepo, t *builder.Target, rr *repo.RemoteRepo, dest string, pkgs ...string) error {
	shoubuild, err := diffPackages(s.Pkgs, rr)
	if err != nil {
		return err
	}
	shoubuild = selectPackages(shoubuild, pkgs)

	if len(shoubuild) == 0 {
		slog.Info("No packages to build")
		return nil
	}

	outDir := path.Join(dest, t.Arch)
	for _, p := range shoubuild {
		pkgbase := p.Base()
		slog.Debug("Starting package build", "pkgbase", pkgbase)
		if err := Package(p, t, outDir); err != nil {
			slog.Error("Package build failed", "pkgbase", pkgbase, "error", err)
			return utils.WrapErr(err, "failed to build package")
		}
		slog.Debug("Package build completed", "pkgbase", pkgbase)
	}
	return nil
}
