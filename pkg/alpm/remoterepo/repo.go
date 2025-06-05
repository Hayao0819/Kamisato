package remoterepo

import "github.com/Hayao0819/Kamisato/pkg/alpm/pkg"

type RemoteRepo struct {
	Name string
	// Url  string
	Pkgs []*pkg.Package
}

func (r *RemoteRepo) PkgByPkgName(pkgname string) *pkg.Package {
	for _, pkg := range r.Pkgs {
		pi := pkg.MustPKGINFO()
		if pi.PkgName == pkgname {
			return pkg
		}
	}
	return nil
}

func (r *RemoteRepo) PkgByPkgBase(pkgbase string) *pkg.Package {
	for _, pkg := range r.Pkgs {
		pi := pkg.MustPKGINFO()
		if pi.PkgBase == pkgbase {
			return pkg
		}
	}
	return nil
}
