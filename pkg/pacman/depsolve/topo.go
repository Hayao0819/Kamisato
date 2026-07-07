package depsolve

import (
	"fmt"
	"slices"
	"strings"
)

// TopoSort returns nodes in dependency-first order; missing deps are added as dep-free nodes and
// independent nodes are sorted lexically for determinism. Returns an error on a cycle.
func TopoSort(nodes []string, deps map[string][]string) ([]string, error) {
	set := nodeSet(nodes, deps)

	// remaining[n] = unemitted dep count; dependents[d] = nodes unblocked when d emits.
	// Self-loops are preserved so they surface as cycles.
	remaining := make(map[string]int, len(set))
	dependents := make(map[string][]string, len(set))
	for n := range set {
		seen := map[string]struct{}{}
		for _, d := range deps[n] {
			if _, dup := seen[d]; dup {
				continue
			}
			seen[d] = struct{}{}
			remaining[n]++
			dependents[d] = append(dependents[d], n)
		}
	}

	order := make([]string, 0, len(set))
	emitted := make(map[string]struct{}, len(set))
	for len(order) < len(set) {
		// Emit the lexically smallest ready node for a stable ordering of independent nodes.
		next, found := "", false
		for n := range set {
			if _, done := emitted[n]; done {
				continue
			}
			if remaining[n] == 0 && (!found || n < next) {
				next, found = n, true
			}
		}
		if !found {
			return nil, cycleError(set, emitted, deps)
		}
		order = append(order, next)
		emitted[next] = struct{}{}
		for _, dep := range dependents[next] {
			remaining[dep]--
		}
	}
	return order, nil
}

// nodeSet returns listed nodes plus any referenced only as deps, so there are no dangling edges.
func nodeSet(nodes []string, deps map[string][]string) map[string]struct{} {
	set := make(map[string]struct{}, len(nodes))
	for _, n := range nodes {
		set[n] = struct{}{}
	}
	for n, ds := range deps {
		set[n] = struct{}{}
		for _, d := range ds {
			set[d] = struct{}{}
		}
	}
	return set
}

// cycleError reports one cycle from the unemitted nodes; walks forward edges depth-first
// from the lexically smallest remaining node until it re-enters a node on the current path.
func cycleError(set, emitted map[string]struct{}, deps map[string][]string) error {
	remaining := make([]string, 0, len(set))
	for n := range set {
		if _, done := emitted[n]; !done {
			remaining = append(remaining, n)
		}
	}
	slices.Sort(remaining)

	onPath := map[string]int{} // node -> its index on the current path
	var path []string
	var walk func(n string) []string
	walk = func(n string) []string {
		if i, ok := onPath[n]; ok {
			return append(slices.Clone(path[i:]), n)
		}
		if _, done := emitted[n]; done {
			return nil
		}
		onPath[n] = len(path)
		path = append(path, n)
		nbrs := slices.Clone(deps[n])
		slices.Sort(nbrs)
		for _, d := range nbrs {
			if _, inSet := set[d]; !inSet {
				continue
			}
			if c := walk(d); c != nil {
				return c
			}
		}
		path = path[:len(path)-1]
		delete(onPath, n)
		return nil
	}
	for _, n := range remaining {
		if c := walk(n); c != nil {
			return fmt.Errorf("depsolve: dependency cycle: %s", strings.Join(c, " -> "))
		}
	}
	return fmt.Errorf("depsolve: dependency cycle among %s", strings.Join(remaining, ", "))
}
