package pkgindex

import (
	"context"
	"sync"
	"testing"

	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

func mkIndex(pkgs ...aurweb.Pkg) *Index {
	i := New()
	byName := make(map[string]aurweb.Pkg, len(pkgs))
	sources := map[string]string{}
	for _, p := range pkgs {
		byName[p.Name] = p
		if p.PackageBase != "" {
			sources[p.PackageBase] = "https://git.example/" + p.PackageBase + ".git"
		}
	}
	i.Replace(byName, sources)
	return i
}

func TestReadsFromSnapshot(t *testing.T) {
	ctx := context.Background()
	i := mkIndex(
		aurweb.Pkg{Name: "alpha", PackageBase: "alpha", Description: "first"},
		aurweb.Pkg{Name: "beta", PackageBase: "beta", Description: "second thing"},
	)

	got, _ := i.Info(ctx, []string{"alpha", "missing"})
	if len(got) != 1 || got[0].Name != "alpha" {
		t.Fatalf("Info = %v, want [alpha]", got)
	}

	// name-desc matches the description too.
	res, _ := i.Search(ctx, aurweb.ByNameDesc, "second")
	if len(res) != 1 || res[0].Name != "beta" {
		t.Fatalf("Search(second) = %v, want [beta]", res)
	}

	sug, _ := i.Suggest(ctx, "al", false)
	if len(sug) != 1 || sug[0] != "alpha" {
		t.Fatalf("Suggest(al) = %v, want [alpha]", sug)
	}

	all, _ := i.All(ctx)
	if len(all) != 2 {
		t.Fatalf("All = %d records, want 2", len(all))
	}

	if u, ok, _ := i.SourceURL(ctx, "beta"); !ok || u != "https://git.example/beta.git" {
		t.Fatalf("SourceURL(beta) = %q, ok=%v", u, ok)
	}
	if _, ok, _ := i.SourceURL(ctx, "nope"); ok {
		t.Fatal("SourceURL(nope) should not resolve")
	}
}

func TestReplaceSwapsAtomically(t *testing.T) {
	ctx := context.Background()
	i := mkIndex(aurweb.Pkg{Name: "old", PackageBase: "old"})

	i.Replace(map[string]aurweb.Pkg{"new": {Name: "new", PackageBase: "new"}}, nil)

	if got, _ := i.Info(ctx, []string{"old"}); len(got) != 0 {
		t.Errorf("old package still visible after Replace: %v", got)
	}
	if got, _ := i.Info(ctx, []string{"new"}); len(got) != 1 {
		t.Errorf("new package not visible after Replace: %v", got)
	}
	// A nil sources map must not panic later reads.
	if _, ok, _ := i.SourceURL(ctx, "new"); ok {
		t.Error("SourceURL should be empty when sources is nil")
	}
}

// TestConcurrentReadDuringRefresh exercises the atomic swap: readers run flat out
// while Replace publishes new snapshots. Under -race this fails if a reader ever
// touched a map a refresh was mutating; the swap guarantees it never does.
func TestConcurrentReadDuringRefresh(t *testing.T) {
	ctx := context.Background()
	i := New()
	i.Replace(map[string]aurweb.Pkg{"seed": {Name: "seed", PackageBase: "seed"}}, nil)

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_, _ = i.Search(ctx, aurweb.ByName, "pkg")
					_, _ = i.Suggest(ctx, "pkg", false)
					_, _ = i.All(ctx)
				}
			}
		}()
	}

	for gen := 0; gen < 200; gen++ {
		next := map[string]aurweb.Pkg{}
		for n := 0; n < 50; n++ {
			name := "pkg" + string(rune('a'+n%26))
			next[name] = aurweb.Pkg{Name: name, PackageBase: name}
		}
		i.Replace(next, nil)
	}

	close(stop)
	wg.Wait()
}
