package repo

import (
	"path"

	"github.com/Morganamilo/go-srcinfo"
)

type PackageSource struct {
	Path    string
	Srcinfo *srcinfo.Srcinfo
}

func GetPackage(dir string) (*PackageSource, error) {
	info, err := srcinfo.ParseFile(path.Join(dir, ".SRCINFO"))
	if err != nil {
		return nil, err
	}

	pkg := new(PackageSource)
	pkg.Path = dir
	pkg.Srcinfo = info

	return pkg, nil
}
