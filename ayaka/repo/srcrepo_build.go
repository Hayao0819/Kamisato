package repo

import (
	"fmt"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/alpmpkg"
	builder "github.com/Hayao0819/Kamisato/ayaka/abs"
	"github.com/samber/lo"
)

func (r *SourceRepo) Build(t *builder.Target, dest string, pkgs ...string) error {
	fulldstdir := path.Join(dest, t.Arch)
	var errs []error
	if err := os.MkdirAll(fulldstdir, 0755); err != nil {
		return err
	}

	var targetPkgs []*alpmpkg.Package
	if len(pkgs) > 0 {
		for _, pkg := range pkgs {
			for _, p := range r.Pkgs {
				pi, err := p.PKGINFO()
				if err != nil {
					continue
				}

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

	for _, pkg := range targetPkgs {
		if err := pkg.Build("archbuild", t, fulldstdir); err != nil {
			// logger.Error(err.Error())
			errs = append(errs, err)
		}

		/*
			if err := r.UploadToBlinky(conf.AppConfig.BlinkyServer, pkg); err != nil {
				logger.Error(err.Error())
			}
		*/

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
