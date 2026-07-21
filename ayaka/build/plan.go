package build

import (
	"cmp"
	"slices"
	"strings"

	"github.com/samber/lo"

	alpm "github.com/Hayao0819/dyalpm"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/pacman/depend"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// CascadeMode selects which build-time dependency updates trigger dependent
// rebuilds in a plan.
type CascadeMode string

const (
	CascadeOff         CascadeMode = "off"
	CascadeMakeDepends CascadeMode = "makedepends"
	CascadeSoname      CascadeMode = "soname"
	CascadeBoth        CascadeMode = "both"
)

// ParseCascadeMode parses a --cascade flag value.
func ParseCascadeMode(s string) (CascadeMode, error) {
	switch m := CascadeMode(s); m {
	case CascadeOff, CascadeMakeDepends, CascadeSoname, CascadeBoth:
		return m, nil
	}
	return "", errors.NewErr("invalid cascade mode: " + s + " (off, makedepends, soname or both)")
}

// Plan is the machine-readable result of `ayaka plan`: what to build this run,
// in which order, and which packages need a pkgrel bump because they rebuild
// without a source change of their own.
type Plan struct {
	Order       []string          `json:"order"`
	Buckets     [][]string        `json:"buckets,omitempty"`
	Reasons     map[string]string `json:"reasons"`
	BumpTargets []string          `json:"bump_targets"`
}

// ComputePlan derives the build set for arch from the source repo and the
// published db alone, so repeated runs against the same db are idempotent.
func ComputePlan(src []*pkg.SourcePackage, rr *repo.RemoteRepo, arch string, cascade CascadeMode, workers int, costs map[string]float64) (*Plan, error) {
	archPkgs := filterByArch(src, arch)
	byBase := lo.KeyBy(archPkgs, (*pkg.SourcePackage).Base)

	reasons := map[string]string{}
	for _, p := range diffPackages(archPkgs, rr) {
		reasons[p.Base()] = "version"
	}

	graph := buildDepGraph(archPkgs, arch)

	if cascade == CascadeMakeDepends || cascade == CascadeBoth {
		// Only a pkgver change seeds the cascade: the cascaded rebuilds bump
		// pkgrel alone, so they cannot re-trigger it and one pass suffices.
		seeds := lo.Filter(lo.Keys(reasons), func(base string, _ int) bool {
			return pkgverChanged(byBase[base], rr)
		})
		for _, dep := range dependentsClosure(graph, seeds) {
			if _, ok := reasons[dep]; !ok {
				reasons[dep] = "makedepends"
			}
		}
	}
	if cascade == CascadeSoname || cascade == CascadeBoth {
		for _, base := range brokenSonameDependents(rr) {
			if _, ok := byBase[base]; !ok {
				continue
			}
			if _, ok := reasons[base]; !ok {
				reasons[base] = "soname"
			}
		}
	}

	plan := &Plan{Order: []string{}, Reasons: reasons, BumpTargets: []string{}}
	if len(reasons) == 0 {
		return plan, nil
	}

	nodes := lo.Keys(reasons)
	subDeps := map[string][]string{}
	for _, n := range nodes {
		for _, d := range graph.Deps(n) {
			if _, ok := reasons[d]; ok {
				subDeps[n] = append(subDeps[n], d)
			}
		}
	}
	order, err := depend.TopoSort(nodes, subDeps)
	if err != nil {
		return nil, err
	}
	plan.Order = order
	plan.BumpTargets = lo.Filter(order, func(n string, _ int) bool {
		return reasons[n] != "version"
	})

	if workers > 0 {
		plan.Buckets = packBuckets(components(nodes, subDeps), order, workers, costs)
	}
	return plan, nil
}

// pkgverChanged reports whether sp's pkgver (epoch included, pkgrel excluded)
// differs from the published one; a package missing from the db counts as
// changed.
func pkgverChanged(sp *pkg.SourcePackage, rr *repo.RemoteRepo) bool {
	rp := rr.PkgByPkgBase(sp.Base())
	if rp == nil {
		return true
	}
	return alpm.VerCmp(pkgverOf(sp.Version()), pkgverOf(rp.Version())) != 0
}

// pkgverOf strips the trailing pkgrel ("1:2.3-4" -> "1:2.3"); pkgver itself
// cannot contain hyphens.
func pkgverOf(v string) string {
	if i := strings.LastIndex(v, "-"); i >= 0 {
		return v[:i]
	}
	return v
}

// dependentsClosure returns the transitive dependents of seeds, sorted.
func dependentsClosure(g *depend.DepGraph, seeds []string) []string {
	seen := map[string]bool{}
	var queue []string
	for _, s := range seeds {
		for _, d := range g.Dependents(s) {
			if !seen[d] {
				seen[d] = true
				queue = append(queue, d)
			}
		}
	}
	for i := 0; i < len(queue); i++ {
		for _, d := range g.Dependents(queue[i]) {
			if !seen[d] {
				seen[d] = true
				queue = append(queue, d)
			}
		}
	}
	slices.Sort(queue)
	return queue
}

// brokenSonameDependents returns the pkgbases whose %DEPENDS% pin a soname the
// db's %PROVIDES% no longer satisfies (sogrep-style, from the db alone).
// Sonames no db package provides are external and skipped.
func brokenSonameDependents(rr *repo.RemoteRepo) []string {
	provides := map[string][]string{}
	for _, p := range rr.Pkgs {
		for _, pr := range p.PKGINFO().Provides {
			c := depend.Parse(pr)
			provides[c.Name] = append(provides[c.Name], c.Ver)
		}
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range rr.Pkgs {
		base := p.Base()
		if base == "" {
			base = p.Name()
		}
		for _, d := range p.PKGINFO().Depend {
			if !strings.Contains(d, ".so") {
				continue
			}
			c := depend.Parse(d)
			vers, intra := provides[c.Name]
			if !intra || sonameSatisfied(c, vers) || seen[base] {
				continue
			}
			seen[base] = true
			out = append(out, base)
		}
	}
	slices.Sort(out)
	return out
}

func sonameSatisfied(c depend.Constraint, vers []string) bool {
	if c.Op == depend.OpAny {
		return true
	}
	for _, v := range vers {
		if v == "" {
			continue
		}
		if ok, err := c.Satisfies(v); err == nil && ok {
			return true
		}
	}
	return false
}

// components returns the connected components of the build set (edges taken
// undirected), lexically ordered by their smallest member.
func components(nodes []string, deps map[string][]string) [][]string {
	adj := map[string][]string{}
	for n, ds := range deps {
		for _, d := range ds {
			adj[n] = append(adj[n], d)
			adj[d] = append(adj[d], n)
		}
	}
	sorted := slices.Clone(nodes)
	slices.Sort(sorted)
	seen := map[string]bool{}
	var comps [][]string
	for _, n := range sorted {
		if seen[n] {
			continue
		}
		seen[n] = true
		comp := []string{}
		queue := []string{n}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			comp = append(comp, cur)
			for _, nb := range adj[cur] {
				if !seen[nb] {
					seen[nb] = true
					queue = append(queue, nb)
				}
			}
		}
		comps = append(comps, comp)
	}
	return comps
}

// packBuckets spreads components over at most workers buckets by summed cost
// (1 when unknown), never splitting a component so intra-component build order
// and publish causality stay within one job. Buckets keep the global order.
func packBuckets(comps [][]string, order []string, workers int, costs map[string]float64) [][]string {
	pos := map[string]int{}
	for i, n := range order {
		pos[n] = i
	}
	type comp struct {
		pkgs []string
		cost float64
	}
	weighted := make([]comp, 0, len(comps))
	for _, c := range comps {
		total := 0.0
		for _, p := range c {
			w, ok := costs[p]
			if !ok || w <= 0 {
				w = 1
			}
			total += w
		}
		weighted = append(weighted, comp{pkgs: c, cost: total})
	}
	slices.SortStableFunc(weighted, func(a, b comp) int { return cmp.Compare(b.cost, a.cost) })

	n := min(workers, len(weighted))
	buckets := make([][]string, n)
	loads := make([]float64, n)
	for _, w := range weighted {
		best := 0
		for i := 1; i < n; i++ {
			if loads[i] < loads[best] {
				best = i
			}
		}
		buckets[best] = append(buckets[best], w.pkgs...)
		loads[best] += w.cost
	}
	for _, b := range buckets {
		slices.SortFunc(b, func(a, c string) int { return cmp.Compare(pos[a], pos[c]) })
	}
	return buckets
}
