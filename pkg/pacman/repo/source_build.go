// SourceRepoのビルド処理
package repo

import (
	"fmt"
	"log/slog"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/internal/utils"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/package"
	"github.com/Hayao0819/Kamisato/pkg/pacman/package/builder"
	"github.com/Hayao0819/Kamisato/pkg/pacman/remote"
	pacman_utils "github.com/Hayao0819/Kamisato/pkg/pacman/utils"
	"github.com/samber/lo"
)

func (r *SourceRepo) Build(t *builder.Target, dest string, pkgs ...string) error {
	fulldstdir := path.Join(dest, t.Arch)
	var errs []error
	if err := os.MkdirAll(fulldstdir, 0755); err != nil {
		return err
	}

	var targetPkgs []*pkg.Package
	if len(pkgs) > 0 {
		for _, name := range pkgs {
			slog.Info("searching for package", "pkg", name)
			for _, p := range r.Pkgs {
				pi, err := p.SRCINFO()
				if err != nil {
					slog.Error("get pkginfo failed", "pkg", name, "err", err)
					continue
				}
				slog.Info("found package", "pkg", pi.PkgBase, "pkgver", pi.PkgVer)

				names := p.Names()
				if name == pi.PkgBase || lo.Contains(names, name) {
					targetPkgs = append(targetPkgs, p)
					break
				}
			}
		}
	} else {
		targetPkgs = r.Pkgs
	}

	if len(targetPkgs) == 0 {
		return fmt.Errorf("no packages found")
	}

	for _, p := range targetPkgs {
		slog.Info("building package", "pkg", p.Names())
		if err := p.Build(t, fulldstdir); err != nil {
			slog.Error("build package failed", "pkg", p.Names(), "err", err)
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		var errstr string
		for _, err := range errs {
			errstr += err.Error() + "\n"
		}
		return fmt.Errorf("errors occurred during build:\n%s", errstr)
	}
	return nil
}

func (s *SourceRepo) DiffBuild(t *builder.Target, rr *remote.RemoteRepo, dest string, pkgs ...string) error {

	var shoubuild []*pkg.Package
	for _, pkg := range s.Pkgs {
		pi := pkg.MustPKGINFO()
		rp := rr.PkgByPkgBase(pi.PkgBase)
		if rp == nil {
			slog.Warn("Package does not exist in remote repository", "pkgbase", pi.PkgBase)
			shoubuild = append(shoubuild, pkg)
			continue
		}
		cmp, err := pacman_utils.VerCmp(pi.PkgVer, rp.MustPKGINFO().PkgVer)
		if err != nil {
			slog.Error("Failed to compare versions", "pkgbase", pi.PkgBase, "error", err)
			return utils.WrapErr(err, "failed to compare package versions")
		}
		if cmp > 0 {
			slog.Debug("Local package is newer", "pkgbase", pi.PkgBase, "local", pi.PkgVer, "remote", rp.MustPKGINFO().PkgVer)
			shoubuild = append(shoubuild, pkg)
		}
	}

	// Filter by specified package names, if any were provided.
	if len(pkgs) > 0 {
		var filtered []*pkg.Package
		for _, p := range shoubuild {
			pi := p.MustPKGINFO()
			names := p.Names()
			for _, name := range pkgs {
				if name == pi.PkgBase || lo.Contains(names, name) {
					filtered = append(filtered, p)
					break
				}
			}
		}
		shoubuild = filtered
	}

	if len(shoubuild) == 0 {
		slog.Info("No packages to build")
		return nil
	}

	outDir := path.Join(dest, s.Config.Name)
	for _, pkg := range shoubuild {
		pkgbase := pkg.MustPKGINFO().PkgBase
		slog.Debug("Starting package build", "pkgbase", pkgbase)
		if err := pkg.Build(t, outDir); err != nil {
			slog.Error("Package build failed", "pkgbase", pkgbase, "error", err)
			return utils.WrapErr(err, "failed to build package")
		}
		slog.Debug("Package build completed", "pkgbase", pkgbase)
	}
	return nil
}
