package service

import (
	"reflect"
	"testing"

	"github.com/Hayao0819/Kamisato/pkg/pacman/depsolve"
	ppkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

type fakeRebuildEnqueuer struct{ order []string }

func (f *fakeRebuildEnqueuer) enqueueRebuild(pkgbase string) error {
	f.order = append(f.order, pkgbase)
	return nil
}

// A soname bump must rebuild the whole transitive reverse-dependency set, in
// dependency order (a rebuilt link before the packages that depend on it), and
// must not rebuild the bumped package itself.
func TestEnqueueRebuildChainUsesDependentsInTopoOrder(t *testing.T) {
	// libbase <- a <- b, and libbase <- c.
	g := depsolve.NewDepGraph(
		[]string{"libbase", "a", "b", "c"},
		map[string][]string{
			"a": {"libbase"},
			"b": {"a"},
			"c": {"libbase"},
		},
	)

	enq := &fakeRebuildEnqueuer{}
	chain, err := enqueueRebuildChain(g, "libbase", enq)
	if err != nil {
		t.Fatalf("enqueueRebuildChain: %v", err)
	}

	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(chain, want) {
		t.Errorf("chain = %v, want %v", chain, want)
	}
	if !reflect.DeepEqual(enq.order, want) {
		t.Errorf("enqueue order = %v, want %v", enq.order, want)
	}
	for _, pb := range chain {
		if pb == "libbase" {
			t.Error("the bumped package must not rebuild itself")
		}
	}
}

func TestRebuildChainNoDependents(t *testing.T) {
	g := depsolve.NewDepGraph([]string{"leaf", "other"}, map[string][]string{"other": {"unrelated"}})
	chain, err := rebuildChain(g, "leaf")
	if err != nil {
		t.Fatalf("rebuildChain: %v", err)
	}
	if len(chain) != 0 {
		t.Errorf("chain = %v, want empty", chain)
	}
}

// repoDepGraph must resolve a soname dependency (libfoo.so=1-64) to the pkgbase
// whose %PROVIDES% offers that soname, so the reverse map links them.
func TestRepoDepGraphResolvesSonameProvider(t *testing.T) {
	provider := raiou.NewPKGINFO()
	provider.PkgName = "foo"
	provider.PkgBase = "foo"
	provider.Provides = []string{"libfoo.so=1-64"}

	consumer := raiou.NewPKGINFO()
	consumer.PkgName = "bar"
	consumer.PkgBase = "bar"
	consumer.Depend = []string{"libfoo.so=1-64"}

	rr := &repo.RemoteRepo{
		Name: "extra",
		Pkgs: []*ppkg.BinaryPackage{
			ppkg.NewBinaryPackage("foo-1-1-x86_64.pkg.tar.zst", provider),
			ppkg.NewBinaryPackage("bar-1-1-x86_64.pkg.tar.zst", consumer),
		},
	}

	g := repoDepGraph(rr)
	if got := g.Dependents("foo"); !reflect.DeepEqual(got, []string{"bar"}) {
		t.Errorf("Dependents(foo) = %v, want [bar]", got)
	}

	enq := &fakeRebuildEnqueuer{}
	chain, err := enqueueRebuildChain(g, "foo", enq)
	if err != nil {
		t.Fatalf("enqueueRebuildChain: %v", err)
	}
	if !reflect.DeepEqual(chain, []string{"bar"}) {
		t.Errorf("chain = %v, want [bar]", chain)
	}
}

func TestFileSonameStoreRoundTrip(t *testing.T) {
	st, err := newFileSonameStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if got, err := st.load("absent"); err != nil || got != nil {
		t.Errorf("load(absent) = %v, %v; want nil, nil", got, err)
	}

	want := []string{"libfoo.so.1", "libbar.so.2"}
	if err := st.save("foo", want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := st.load("foo")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("load(foo) = %v, want %v", got, want)
	}
}

func TestFileSonameStoreRejectsUnsafePkgbase(t *testing.T) {
	st, err := newFileSonameStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"../escape", "a/b", ""} {
		if err := st.save(bad, []string{"x"}); err == nil {
			t.Errorf("save(%q) succeeded, want an error", bad)
		}
	}
}

func TestDepName(t *testing.T) {
	cases := map[string]string{
		"libfoo.so=1-64": "libfoo.so",
		"bar>=2.0":       "bar",
		"baz<3":          "baz",
		"plain":          "plain",
	}
	for in, want := range cases {
		if got := depName(in); got != want {
			t.Errorf("depName(%q) = %q, want %q", in, got, want)
		}
	}
}
