package repo

import (
	"path"

	"github.com/Morganamilo/go-srcinfo"
)

type Package struct {
	Path    string
	Srcinfo *srcinfo.Srcinfo
}

func GetPackage(dir string) (*Package, error) {
	info, err := srcinfo.ParseFile(path.Join(dir, ".SRCINFO"))
	if err != nil {
		return nil, err
	}

	pkg := new(Package)
	pkg.Path = dir
	pkg.Srcinfo = info

	return pkg, nil
}
