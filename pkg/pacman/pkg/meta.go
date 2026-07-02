package pkg

import (
	"github.com/samber/lo"
)

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

func (p *BinaryPackage) Name() string {
	return p.info.PkgName
}

func (p *BinaryPackage) Base() string {
	return p.info.PkgBase
}

// Version returns the pkgver (the full version recorded in .PKGINFO).
func (p *BinaryPackage) Version() string {
	return p.info.PkgVer
}

func (p *BinaryPackage) Arch() string {
	return p.info.Arch
}
