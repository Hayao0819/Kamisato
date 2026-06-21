// Build logic for SourceRepo.
package repo

import (
	"fmt"
	"log/slog"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/pacman/alpm"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/samber/lo"
)

func (r *SourceRepo) Build(t *builder.Target, dest string, pkgs ...string) error {
	fulldstdir := path.Join(dest, t.Arch)
	var errs []error
	if err := os.MkdirAll(fulldstdir, 0755); err != nil {
		return err
	}

	var targetPkgs []*pkg.SourcePackage
	if len(pkgs) > 0 {
		for _, name := range pkgs {
			slog.Info("searching for package", "pkg", name)
			for _, p := range r.Pkgs {
				slog.Info("found package", "pkg", p.Base(), "pkgver", p.Version())

				names := p.Names()
				if name == p.Base() || lo.Contains(names, name) {
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

func (s *SourceRepo) DiffBuild(t *builder.Target, rr *RemoteRepo, dest string, pkgs ...string) error {

	var shoubuild []*pkg.SourcePackage
	for _, sp := range s.Pkgs {
		rp := rr.PkgByPkgBase(sp.Base())
		if rp == nil {
			slog.Warn("Package does not exist in remote repository", "pkgbase", sp.Base())
			shoubuild = append(shoubuild, sp)
			continue
		}
		cmp, err := alpm.VerCmp(sp.Version(), rp.Version())
		if err != nil {
			slog.Error("Failed to compare versions", "pkgbase", sp.Base(), "error", err)
			return utils.WrapErr(err, "failed to compare package versions")
		}
		if cmp > 0 {
			slog.Debug("Local package is newer", "pkgbase", sp.Base(), "local", sp.Version(), "remote", rp.Version())
			shoubuild = append(shoubuild, sp)
		}
	}

	// Filter by specified package names, if any were provided.
	if len(pkgs) > 0 {
		var filtered []*pkg.SourcePackage
		for _, p := range shoubuild {
			names := p.Names()
			for _, name := range pkgs {
				if name == p.Base() || lo.Contains(names, name) {
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
	for _, p := range shoubuild {
		pkgbase := p.Base()
		slog.Debug("Starting package build", "pkgbase", pkgbase)
		if err := p.Build(t, outDir); err != nil {
			slog.Error("Package build failed", "pkgbase", pkgbase, "error", err)
			return utils.WrapErr(err, "failed to build package")
		}
		slog.Debug("Package build completed", "pkgbase", pkgbase)
	}
	return nil
}
