package repo

import (
	"fmt"
	"log/slog"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/pkg/alpm/builder"
	"github.com/Hayao0819/Kamisato/pkg/alpm/pkg"
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
		for _, pkg := range pkgs {
			slog.Info("searching for package", "pkg", pkg)
			for _, p := range r.Pkgs {
				pi, err := p.SRCINFO()
				if err != nil {
					slog.Error("get pkginfo failed", "pkg", pkg, "err", err)
					continue
				}
				slog.Info("found package", "pkg", pi.PkgBase, "pkgver", pi.PkgVer)

				names := p.Names()
				if pkg == pi.PkgBase || lo.Contains(names, pkg) {
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

	for _, pkg := range targetPkgs {
		slog.Info("building package", "pkg", pkg.Names())
		if err := pkg.Build(t, fulldstdir); err != nil {
			slog.Error("build package failed", "pkg", pkg.Names(), "err", err)
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
