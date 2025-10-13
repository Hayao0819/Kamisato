// リモートリポジトリ型
package remote

import pkg "github.com/Hayao0819/Kamisato/pkg/pacman/package"

type RemoteRepo struct {
	Name   string
	Server string
	Pkgs   []*pkg.Package
}

func (r *RemoteRepo) PkgByPkgName(pkgname string) *pkg.Package {
	for _, p := range r.Pkgs {
		pi := p.MustPKGINFO()
		if pi.PkgName == pkgname {
			return p
		}
	}
	return nil
}

func (r *RemoteRepo) PkgByPkgBase(pkgbase string) *pkg.Package {
	for _, p := range r.Pkgs {
		pi := p.MustPKGINFO()
		if pi.PkgBase == pkgbase {
			return p
		}
	}
	return nil
}
