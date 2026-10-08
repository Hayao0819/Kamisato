package source

import (
	"cmp"
	"log/slog"
	"slices"

	"github.com/Hayao0819/Kamisato/internal/pacman/depend"
	"github.com/Hayao0819/Kamisato/internal/pacman/pkg"
	"github.com/samber/lo"
)

// SelectPackages returns the packages in pkgs whose pkgbase or any sub-package
// name is in names; all of them when names is empty.
func SelectPackages(pkgs []*pkg.SourcePackage, names []string) []*pkg.SourcePackage {
	if len(names) == 0 {
		return pkgs
	}
	var selected []*pkg.SourcePackage
	for _, name := range names {
		for _, p := range pkgs {
			if name == p.Base() || lo.Contains(p.Names(), name) {
				selected = append(selected, p)
				break
			}
		}
	}
	return selected
}

// FilterByArch drops packages whose arch=() excludes arch ("any" matches all), so
// a mixed-arch source repo builds only what each PKGBUILD supports.
func FilterByArch(pkgs []*pkg.SourcePackage, arch string) []*pkg.SourcePackage {
	var kept []*pkg.SourcePackage
	for _, p := range pkgs {
		if p.SupportsArch(arch) {
			kept = append(kept, p)
			continue
		}
		slog.Info("skipping package: arch not supported", "pkgbase", p.Base(), "arch", arch, "supports", p.Arches())
	}
	return kept
}

// BuildDependencyGraph resolves each package's makedepends/checkdepends to the source
// package providing them for arch. Runtime depends are not edges: installing a
// newer dependency does not invalidate a dependent's binary, only build-time
// deps order builds and drive the rebuild cascade.
func BuildDependencyGraph(pkgs []*pkg.SourcePackage, arch string) *depend.Graph {
	// Real pkgnames are registered before any provides so a provides entry can
	// never shadow an actual package; without this the graph would depend on
	// directory iteration order.
	provider := map[string]string{}
	for _, p := range pkgs {
		for _, n := range p.Names() {
			provider[n] = p.Base()
		}
	}
	for _, p := range pkgs {
		for _, pr := range p.Provides(arch) {
			name := depend.Parse(pr).Name
			if _, taken := provider[name]; !taken {
				provider[name] = p.Base()
			}
		}
	}
	deps := map[string][]string{}
	for _, p := range pkgs {
		for _, d := range append(p.MakeDepends(arch), p.CheckDepends(arch)...) {
			if prov, ok := provider[depend.Parse(d).Name]; ok && prov != p.Base() {
				deps[p.Base()] = append(deps[p.Base()], prov)
			}
		}
	}
	return depend.NewGraph(lo.Map(pkgs, func(p *pkg.SourcePackage, _ int) string { return p.Base() }), deps)
}

// OrderByDeps sorts pkgs dependencies-first for arch so a publish-as-you-build
// run can feed later builds; the incoming order is kept on a dependency cycle.
func OrderByDeps(pkgs []*pkg.SourcePackage, arch string) []*pkg.SourcePackage {
	order, err := BuildDependencyGraph(pkgs, arch).BuildOrder()
	if err != nil {
		slog.Warn("keeping given package order", "err", err)
		return pkgs
	}
	pos := make(map[string]int, len(order))
	for i, n := range order {
		pos[n] = i
	}
	sorted := slices.Clone(pkgs)
	slices.SortStableFunc(sorted, func(a, b *pkg.SourcePackage) int {
		return cmp.Compare(pos[a.Base()], pos[b.Base()])
	})
	return sorted
}
