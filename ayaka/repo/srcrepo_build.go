package repo

import (
	"fmt"
	"log/slog"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/alpm"
	"github.com/Hayao0819/Kamisato/alpm/builder"
	"github.com/samber/lo"
)

func (r *SourceRepo) Build(t *builder.Target, dest string, pkgs ...string) error {
	fulldstdir := path.Join(dest, t.Arch)
	var errs []error
	if err := os.MkdirAll(fulldstdir, 0755); err != nil {
		return err
	}

	var targetPkgs []*alpm.Package
	if len(pkgs) > 0 {
		for _, pkg := range pkgs {
			slog.Info("searching for package", "pkg", pkg)
			for _, p := range r.Pkgs {
				pi, err := p.SRCINFO()
				if err != nil {
					slog.Error("get pkginfo failed", "pkg", pkg, "err", err)
					continue
				}
				slog.Info("found package", "pkg", pi.Pkgbase, "pkgver", pi.Pkgver)

				names := p.Names()
				if pkg == pi.Pkgbase || lo.Contains(names, pkg) {
					targetPkgs = append(targetPkgs, p)
					break
				}
			}
		}
	} else {
		targetPkgs = r.Pkgs
	}

	for _, pkg := range targetPkgs {
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
