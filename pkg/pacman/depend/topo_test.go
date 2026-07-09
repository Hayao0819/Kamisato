package depend_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/pkg/pacman/depend"
)

func TestTopoSortLinearChain(t *testing.T) {
	// A depends on B depends on C: build the deepest dependency first.
	order, err := depend.TopoSort([]string{"A", "B", "C"}, map[string][]string{
		"A": {"B"},
		"B": {"C"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(order, []string{"C", "B", "A"}) {
		t.Fatalf("order = %v, want [C B A]", order)
	}
}

func TestTopoSortDiamond(t *testing.T) {
	// A -> {B, C} -> D: D first, its two dependents in lexical order, then A.
	order, err := depend.TopoSort([]string{"A", "B", "C", "D"}, map[string][]string{
		"A": {"B", "C"},
		"B": {"D"},
		"C": {"D"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(order, []string{"D", "B", "C", "A"}) {
		t.Fatalf("order = %v, want [D B C A]", order)
	}
}

func TestTopoSortIndependentSetIsLexical(t *testing.T) {
	// No edges: the order is fully determined by the lexical tie-break and must
	// not depend on the input or map iteration order.
	order, err := depend.TopoSort([]string{"c", "a", "b"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(order, []string{"a", "b", "c"}) {
		t.Fatalf("order = %v, want [a b c]", order)
	}
}

func TestTopoSortIncludesReferencedDep(t *testing.T) {
	// A dependency named only in the edges (not in nodes) is still built.
	order, err := depend.TopoSort([]string{"A"}, map[string][]string{"A": {"B"}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(order, []string{"B", "A"}) {
		t.Fatalf("order = %v, want [B A]", order)
	}
}

func TestTopoSortCycleErrors(t *testing.T) {
	_, err := depend.TopoSort([]string{"A", "B"}, map[string][]string{
		"A": {"B"},
		"B": {"A"},
	})
	if err == nil {
		t.Fatal("expected a cycle error")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("error should report a cycle, got %v", err)
	}
	// The named cycle must mention both nodes on it.
	if !strings.Contains(err.Error(), "A") || !strings.Contains(err.Error(), "B") {
		t.Fatalf("cycle error should name the nodes, got %v", err)
	}
}

func TestTopoSortSelfLoopErrors(t *testing.T) {
	if _, err := depend.TopoSort([]string{"A"}, map[string][]string{"A": {"A"}}); err == nil {
		t.Fatal("expected a cycle error for a self-loop")
	}
}
