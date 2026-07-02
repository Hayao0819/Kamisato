package depsolve_test

import (
	"slices"
	"testing"

	"github.com/Hayao0819/Kamisato/pkg/pacman/depsolve"
)

func TestDepGraphForwardAndReverse(t *testing.T) {
	// A -> {B, C}, B -> D, C -> D.
	g := depsolve.NewDepGraph([]string{"A", "B", "C", "D"}, map[string][]string{
		"A": {"C", "B"}, // unsorted input: accessors must still return sorted
		"B": {"D"},
		"C": {"D"},
	})

	if got := g.Deps("A"); !slices.Equal(got, []string{"B", "C"}) {
		t.Errorf("Deps(A) = %v, want [B C]", got)
	}
	if got := g.Deps("D"); len(got) != 0 {
		t.Errorf("Deps(D) = %v, want none", got)
	}
	// D is depended on by both B and C.
	if got := g.Dependents("D"); !slices.Equal(got, []string{"B", "C"}) {
		t.Errorf("Dependents(D) = %v, want [B C]", got)
	}
	if got := g.Dependents("A"); len(got) != 0 {
		t.Errorf("Dependents(A) = %v, want none (nothing depends on the root)", got)
	}
	if got := g.Nodes(); !slices.Equal(got, []string{"A", "B", "C", "D"}) {
		t.Errorf("Nodes() = %v, want [A B C D]", got)
	}
}

func TestDepGraphIncludesReferencedDep(t *testing.T) {
	// E is named only as a dependency, yet must appear as a node with A as its
	// dependent so the reverse map is complete for rebuild-chain lookups.
	g := depsolve.NewDepGraph([]string{"A"}, map[string][]string{"A": {"E"}})
	if got := g.Dependents("E"); !slices.Equal(got, []string{"A"}) {
		t.Errorf("Dependents(E) = %v, want [A]", got)
	}
}

func TestDepGraphBuildOrderMatchesTopoSort(t *testing.T) {
	g := depsolve.NewDepGraph([]string{"A", "B", "C", "D"}, map[string][]string{
		"A": {"B", "C"},
		"B": {"D"},
		"C": {"D"},
	})
	order, err := g.BuildOrder()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(order, []string{"D", "B", "C", "A"}) {
		t.Fatalf("BuildOrder = %v, want [D B C A]", order)
	}
}

func TestDepGraphDropsSelfLoop(t *testing.T) {
	// A self-dependency must not appear as its own edge; BuildOrder stays acyclic.
	g := depsolve.NewDepGraph([]string{"A"}, map[string][]string{"A": {"A"}})
	if got := g.Deps("A"); len(got) != 0 {
		t.Errorf("Deps(A) = %v, want no self-edge", got)
	}
	order, err := g.BuildOrder()
	if err != nil {
		t.Fatalf("self-loop dropped from the graph should build cleanly, got %v", err)
	}
	if !slices.Equal(order, []string{"A"}) {
		t.Fatalf("BuildOrder = %v, want [A]", order)
	}
}
