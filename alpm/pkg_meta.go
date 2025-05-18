package alpm

import (
	"bytes"
	"fmt"
	"os/exec"
	"path"
	"strings"

	"github.com/samber/lo"
)

func (p *Package) Names() []string {

	names := make([]string, 0)
	if p.pkginfo != nil {
		names = append(names, p.pkginfo.PkgBase, p.pkginfo.PkgName)
	}
	if p.srcinfo != nil {
		names = append(names, p.srcinfo.Pkgbase)
		for _, pkg := range p.srcinfo.Packages {
			names = append(names, pkg.Pkgname)
		}
	}
	names = lo.Uniq(names)
	return names

}

func (p *Package) GetPkgFileNames() ([]string, error) {
	stdout := new(bytes.Buffer)
	cmd := exec.Command("makepkg", "--packagelist", "OPTIONS=-debug")
	cmd.Dir = p.path

	cmd.Stdout = stdout
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	l := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(l) == 0 {
		return nil, fmt.Errorf("no package found")
	}
	pkgs := make([]string, len(l))
	for i, pkg := range l {
		pkgs[i] = path.Base(strings.TrimSpace(pkg))
	}

	return pkgs, nil
}
