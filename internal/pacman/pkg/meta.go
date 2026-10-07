package pkg

import (
	"slices"

	"github.com/samber/lo"
)

func (p *SourcePackage) Base() string {
	return p.info.PkgBase
}

// Version returns the full version in epoch:pkgver-pkgrel form (for vercmp).
func (p *SourcePackage) Version() string {
	return p.info.Version()
}

// Pkgver returns the bare pkgver, without epoch or pkgrel (for comparing
// against upstream versions).
func (p *SourcePackage) Pkgver() string {
	return p.info.PkgVer
}

func (p *SourcePackage) Sources(arch string) []string {
	return lo.Uniq(p.info.Source.ForArch(arch))
}

// Names returns the pkgbase and each sub-package name.
func (p *SourcePackage) Names() []string {
	names := []string{p.info.PkgBase}
	for _, pkg := range p.info.Packages {
		names = append(names, pkg.PkgName)
	}
	return lo.Uniq(names)
}

func (p *SourcePackage) OutputNames(arch string) []string {
	var names []string
	for _, output := range p.info.SplitPackages() {
		if supportsPackageArch(output.PkgArch, arch) {
			names = append(names, output.PkgName)
		}
	}
	if len(names) == 0 && len(p.info.Packages) == 0 && p.info.PkgName != "" && supportsPackageArch(p.info.PkgArch, arch) {
		names = append(names, p.info.PkgName)
	}
	slices.Sort(names)
	return slices.Compact(names)
}

func (p *SourcePackage) BuildDepends(arch string) []string {
	deps := append([]string{}, p.info.MakeDepends.ForArch(arch)...)
	deps = append(deps, p.info.CheckDepends.ForArch(arch)...)
	for _, output := range p.info.SplitPackages() {
		if supportsPackageArch(output.PkgArch, arch) {
			deps = append(deps, output.Depends.ForArch(arch)...)
		}
	}
	if len(p.info.Packages) == 0 && supportsPackageArch(p.info.PkgArch, arch) {
		deps = append(deps, p.info.Depends.ForArch(arch)...)
	}
	slices.Sort(deps)
	return slices.Compact(deps)
}

func (p *SourcePackage) OutputProvides(arch string) []string {
	var provides []string
	for _, output := range p.info.SplitPackages() {
		if supportsPackageArch(output.PkgArch, arch) {
			provides = append(provides, output.Provides.ForArch(arch)...)
		}
	}
	if len(p.info.Packages) == 0 && supportsPackageArch(p.info.PkgArch, arch) {
		provides = append(provides, p.info.Provides.ForArch(arch)...)
	}
	slices.Sort(provides)
	return slices.Compact(provides)
}

func supportsPackageArch(arches []string, arch string) bool {
	if len(arches) == 0 {
		return true
	}
	return slices.Contains(arches, "any") || slices.Contains(arches, arch)
}

// Arches returns the pkgbase and sub-package arch=() values.
func (p *SourcePackage) Arches() []string {
	arches := append([]string{}, p.info.PkgArch...)
	for _, pkg := range p.info.Packages {
		arches = append(arches, pkg.PkgArch...)
	}
	return lo.Uniq(arches)
}

// Depends returns the runtime depends for arch across the pkgbase and every
// sub-package.
func (p *SourcePackage) Depends(arch string) []string {
	out := p.info.SrcinfoPackage.Depends.ForArch(arch)
	for _, sub := range p.info.Packages {
		out = append(out, sub.Depends.ForArch(arch)...)
	}
	return lo.Uniq(out)
}

// MakeDepends returns the makedepends for arch (pkgbase-level, as in .SRCINFO).
func (p *SourcePackage) MakeDepends(arch string) []string {
	return lo.Uniq(p.info.MakeDepends.ForArch(arch))
}

// CheckDepends returns the checkdepends for arch (pkgbase-level, as in .SRCINFO).
func (p *SourcePackage) CheckDepends(arch string) []string {
	return lo.Uniq(p.info.CheckDepends.ForArch(arch))
}

// Provides returns what the pkgbase and every sub-package provide for arch.
func (p *SourcePackage) Provides(arch string) []string {
	out := p.info.SrcinfoPackage.Provides.ForArch(arch)
	for _, sub := range p.info.Packages {
		out = append(out, sub.Provides.ForArch(arch)...)
	}
	return lo.Uniq(out)
}

// SupportsArch reports whether the package builds for arch; "any" and an
// undeclared arch match everything.
func (p *SourcePackage) SupportsArch(arch string) bool {
	arches := p.Arches()
	if len(arches) == 0 {
		return true
	}
	for _, a := range arches {
		if a == "any" || a == arch {
			return true
		}
	}
	return false
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
