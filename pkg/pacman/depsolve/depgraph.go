package depsolve

import "slices"

// DepGraph holds both forward and reverse edges of a resolved dependency graph.
// Reverse edges power rebuild-chain detection: when a package changes, its dependents are what must rebuild.
type DepGraph struct {
	nodes   []string
	forward map[string][]string
	reverse map[string][]string
}

// NewDepGraph builds forward and reverse dependency maps from pkgs and deps;
// edges referencing packages absent from pkgs are still added. Self-loops are dropped; edge lists are deduped and sorted.
func NewDepGraph(pkgs []string, deps map[string][]string) *DepGraph {
	set := nodeSet(pkgs, deps)
	forward := make(map[string]map[string]struct{}, len(set))
	reverse := make(map[string]map[string]struct{}, len(set))
	for n := range set {
		forward[n] = map[string]struct{}{}
		reverse[n] = map[string]struct{}{}
	}
	for n, ds := range deps {
		for _, d := range ds {
			if d == n {
				continue
			}
			forward[n][d] = struct{}{}
			reverse[d][n] = struct{}{}
		}
	}

	g := &DepGraph{
		nodes:   make([]string, 0, len(set)),
		forward: make(map[string][]string, len(set)),
		reverse: make(map[string][]string, len(set)),
	}
	for n := range set {
		g.nodes = append(g.nodes, n)
		g.forward[n] = sortedKeys(forward[n])
		g.reverse[n] = sortedKeys(reverse[n])
	}
	slices.Sort(g.nodes)
	return g
}

// Nodes returns every package in the graph, sorted.
func (g *DepGraph) Nodes() []string { return slices.Clone(g.nodes) }

// Deps returns the packages pkg directly depends on (forward edges), sorted.
func (g *DepGraph) Deps(pkg string) []string { return slices.Clone(g.forward[pkg]) }

// Dependents returns the packages that directly depend on pkg (reverse edges),
// sorted. This is the seam soname/rebuild-chain work builds on.
func (g *DepGraph) Dependents(pkg string) []string { return slices.Clone(g.reverse[pkg]) }

// BuildOrder returns the graph's packages in dependency order (dependencies
// first), erroring on a cycle. It is the topological sort over the same edges.
func (g *DepGraph) BuildOrder() ([]string, error) { return TopoSort(g.nodes, g.forward) }

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
