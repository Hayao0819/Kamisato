package repo

import (
	"cmp"
	"log/slog"
	"slices"

	"github.com/samber/lo"

	alpm "github.com/Hayao0819/dyalpm"

	"github.com/Hayao0819/Kamisato/pkg/pacman/depend"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
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

// DiffPackages returns the source packages that are newer than (or missing
// from) the remote repo rr.
func DiffPackages(src []*pkg.SourcePackage, rr *RemoteRepo) []*pkg.SourcePackage {
	var toBuild []*pkg.SourcePackage
	for _, sp := range src {
		rp := rr.PkgByPkgBase(sp.Base())
		if rp == nil {
			slog.Warn("Package does not exist in remote repository", "pkgbase", sp.Base())
			toBuild = append(toBuild, sp)
			continue
		}
		if alpm.VerCmp(sp.Version(), rp.Version()) > 0 {
			slog.Debug("Local package is newer", "pkgbase", sp.Base(), "local", sp.Version(), "remote", rp.Version())
			toBuild = append(toBuild, sp)
		}
	}
	return toBuild
}

// BuildDepGraph resolves each package's makedepends/checkdepends to the source
// package providing them for arch. Runtime depends are not edges: installing a
// newer dependency does not invalidate a dependent's binary, only build-time
// deps order builds and drive the rebuild cascade.
func BuildDepGraph(pkgs []*pkg.SourcePackage, arch string) *depend.DepGraph {
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

// PrunablePackages returns the pkgnames present in the remote repo that are not in
// desired — i.e. packages whose PKGBUILD was removed from the source repo.
func PrunablePackages(desired []string, rr *RemoteRepo) []string {
	set := make(map[string]struct{}, len(desired))
	for _, n := range desired {
		set[n] = struct{}{}
	}
	var prune []string
	for _, bp := range rr.Pkgs {
		if _, ok := set[bp.Name()]; !ok {
			prune = append(prune, bp.Name())
		}
	}
	return prune
}

// OrderByDeps sorts pkgs dependencies-first for arch so a publish-as-you-build
// run can feed later builds; the incoming order is kept on a dependency cycle.
func OrderByDeps(pkgs []*pkg.SourcePackage, arch string) []*pkg.SourcePackage {
	order, err := BuildDepGraph(pkgs, arch).BuildOrder()
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
