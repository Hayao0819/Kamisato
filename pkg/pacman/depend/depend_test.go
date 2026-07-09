package depend_test

import (
	"context"
	"slices"
	"testing"

	"github.com/Hayao0819/Kamisato/pkg/pacman/depend"
)

type fakeRepo struct{ have map[string]bool }

func (f fakeRepo) Unsatisfied(deps []string) ([]string, error) {
	var miss []string
	for _, d := range deps {
		if !f.have[depend.Parse(d).Name] {
			miss = append(miss, d)
		}
	}
	return miss, nil
}

type fakeAUR struct {
	byName   map[string]depend.Pkg
	provides map[string]depend.Pkg
}

func (f fakeAUR) Info(_ context.Context, names []string) ([]depend.Pkg, error) {
	var out []depend.Pkg
	for _, n := range names {
		if p, ok := f.byName[n]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f fakeAUR) ProvidedBy(_ context.Context, name string) (*depend.Pkg, error) {
	if p, ok := f.provides[name]; ok {
		return &p, nil
	}
	return nil, nil
}

func bases(order []depend.Pkg) []string {
	out := make([]string, len(order))
	for i, p := range order {
		out[i] = p.PackageBase
	}
	return out
}

func idx(order []depend.Pkg, base string) int {
	return slices.IndexFunc(order, func(p depend.Pkg) bool { return p.PackageBase == base })
}

func mk(name string, deps ...string) depend.Pkg {
	return depend.Pkg{Name: name, PackageBase: name, Deps: deps}
}

func TestResolveLinear(t *testing.T) {
	aur := fakeAUR{byName: map[string]depend.Pkg{
		"A": mk("A", "B"),
		"B": mk("B"),
	}}
	order, err := depend.Resolve(context.Background(), []string{"A"}, fakeRepo{}, aur)
	if err != nil {
		t.Fatal(err)
	}
	if got := bases(order); !slices.Equal(got, []string{"B", "A"}) {
		t.Fatalf("order = %v, want [B A]", got)
	}
}

func TestResolveDiamondBuildsSharedDepOnce(t *testing.T) {
	aur := fakeAUR{byName: map[string]depend.Pkg{
		"A": mk("A", "B", "C"),
		"B": mk("B", "D"),
		"C": mk("C", "D"),
		"D": mk("D"),
	}}
	order, err := depend.Resolve(context.Background(), []string{"A"}, fakeRepo{}, aur)
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 4 {
		t.Fatalf("want 4 unique bases, got %v", bases(order))
	}
	if idx(order, "D") > idx(order, "B") || idx(order, "D") > idx(order, "C") || idx(order, "A") != 3 {
		t.Fatalf("bad build order: %v", bases(order))
	}
}

func TestResolveSkipsRepoSatisfied(t *testing.T) {
	aur := fakeAUR{byName: map[string]depend.Pkg{"A": mk("A", "B")}}
	repo := fakeRepo{have: map[string]bool{"B": true}} // B comes from a repo
	order, err := depend.Resolve(context.Background(), []string{"A"}, repo, aur)
	if err != nil {
		t.Fatal(err)
	}
	if got := bases(order); !slices.Equal(got, []string{"A"}) {
		t.Fatalf("order = %v, want [A]", got)
	}
}

func TestResolveProvides(t *testing.T) {
	aur := fakeAUR{
		byName:   map[string]depend.Pkg{},
		provides: map[string]depend.Pkg{"cc": mk("gcc-custom")},
	}
	order, err := depend.Resolve(context.Background(), []string{"cc"}, fakeRepo{}, aur)
	if err != nil {
		t.Fatal(err)
	}
	if got := bases(order); !slices.Equal(got, []string{"gcc-custom"}) {
		t.Fatalf("order = %v, want [gcc-custom]", got)
	}
}

func TestResolveCycle(t *testing.T) {
	aur := fakeAUR{byName: map[string]depend.Pkg{
		"A": mk("A", "B"),
		"B": mk("B", "A"),
	}}
	if _, err := depend.Resolve(context.Background(), []string{"A"}, fakeRepo{}, aur); err == nil {
		t.Fatal("expected a cycle error")
	}
}

func TestResolveSkipsNonAUR(t *testing.T) {
	// A dependency not in the AUR is provided by the build environment's repos:
	// it is skipped, not treated as an error.
	order, err := depend.Resolve(context.Background(), []string{"nope"}, fakeRepo{}, fakeAUR{})
	if err != nil {
		t.Fatalf("non-AUR dep should be skipped, got error: %v", err)
	}
	if len(order) != 0 {
		t.Fatalf("expected empty build order, got %v", order)
	}
}
