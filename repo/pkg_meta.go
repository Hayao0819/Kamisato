package repo

import (
	"bytes"
	"fmt"
	"os/exec"
	"path"
	"strings"

	"github.com/samber/lo"
)

func (p *Package) Names() []string {
	names := []string{
		p.Srcinfo.Pkgbase,
	}
	for _, pkg := range p.Srcinfo.Packages {
		names = append(names, pkg.Pkgname)
	}

	names = lo.Uniq(names)

	return names
}

func (p *Package) GetPkgFileNames() ([]string, error) {
	stdout := new(bytes.Buffer)
	cmd := exec.Command("makepkg", "--packagelist")
	cmd.Dir = p.Path
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
