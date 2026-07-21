package build

import (
	"github.com/samber/lo"

	"github.com/Hayao0819/Kamisato/pkg/pacman/depend"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
)

// buildDepGraph resolves each package's makedepends/checkdepends to the source
// package providing them for arch. Runtime depends are not edges: installing a
// newer dependency does not invalidate a dependent's binary, only build-time
// deps order builds and drive the rebuild cascade.
func buildDepGraph(pkgs []*pkg.SourcePackage, arch string) *depend.DepGraph {
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
	return depend.NewDepGraph(lo.Map(pkgs, func(p *pkg.SourcePackage, _ int) string { return p.Base() }), deps)
}
