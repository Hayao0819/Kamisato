package depsolve

import (
	"fmt"
	"slices"
	"strings"
)

// TopoSort orders nodes so every node comes after all the nodes it depends on
// (dependencies first) — the build order for a whole set of packages, the
// equivalent of Arch's rebuild-order tooling. deps maps a node to the nodes it
// depends on; a dependency referenced there but absent from nodes is added as its
// own dependency-free node so no edge dangles. Independent nodes come out in
// lexical order, making the result deterministic. It returns an error naming a
// cycle when the edges are not acyclic (it never loops forever on one).
func TopoSort(nodes []string, deps map[string][]string) ([]string, error) {
	set := nodeSet(nodes, deps)

	// remaining[n] counts n's not-yet-emitted dependencies; dependents[d] lists the
	// nodes that depend on d, so emitting d can unblock them. Duplicate edges are
	// deduped; a self-loop is left in place so it surfaces as a cycle below.
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
		// Emit the lexically smallest node with no pending dependency, so
		// independent nodes come out in a stable order.
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

// nodeSet returns the full vertex set: the listed nodes plus any node referenced
// only as a dependency, so the graph has no dangling edges.
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

// cycleError names one dependency cycle among the still-unemitted nodes (those
// are exactly the nodes on or blocked by a cycle). It walks forward edges
// depth-first from the lexically smallest remaining node until it re-enters a
// node already on the current path, then reports that path.
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
