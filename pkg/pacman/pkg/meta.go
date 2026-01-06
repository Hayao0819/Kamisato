// Accessors for package metadata.
package pkg

import (
	"bytes"
	"fmt"
	"os/exec"
	"path"
	"strings"

	"github.com/samber/lo"
)

// --- SourcePackage ---

// Base returns the pkgbase.
func (p *SourcePackage) Base() string {
	return p.info.PkgBase
}

// Version returns the full version in epoch:pkgver-pkgrel form (for vercmp).
func (p *SourcePackage) Version() string {
	v := p.info.PkgVer
	if p.info.PkgRel != "" {
		v += "-" + p.info.PkgRel
	}
	if p.info.PkgEpoch != "" {
		v = p.info.PkgEpoch + ":" + v
	}
	return v
}

// Names returns the pkgbase and each sub-package name.
func (p *SourcePackage) Names() []string {
	names := []string{p.info.PkgBase}
	for _, pkg := range p.info.Packages {
		names = append(names, pkg.PkgName)
	}
	return lo.Uniq(names)
}

// PkgFileNames returns the build-output file names from makepkg --packagelist.
func (p *SourcePackage) PkgFileNames() ([]string, error) {
	stdout := new(bytes.Buffer)
	cmd := exec.Command("makepkg", "--packagelist")
	cmd.Dir = p.dir
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

// --- BinaryPackage ---

// Name returns the pkgname.
func (p *BinaryPackage) Name() string {
	return p.info.PkgName
}

// Base returns the pkgbase.
func (p *BinaryPackage) Base() string {
	return p.info.PkgBase
}

// Version returns the pkgver (the full version recorded in .PKGINFO).
func (p *BinaryPackage) Version() string {
	return p.info.PkgVer
}

// Arch returns the package architecture.
func (p *BinaryPackage) Arch() string {
	return p.info.Arch
}
