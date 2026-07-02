// Package pkgindex is a concurrency-safe in-memory aurweb index: a pkgname->record
// map with prefix suggest and a pkgbase->clone-URL side table. overlay.Registry and
// ayato.Source both embed it so the read-side Backend methods (Info/Search/Suggest/
// All/SourceURL) live in one place and cannot drift apart.
package pkgindex

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

// Index holds one snapshot of packages keyed by pkgname plus a pkgbase->clone-URL
// table, guarded by its own RWMutex. The zero value is not usable; call New.
type Index struct {
	mu      sync.RWMutex
	byName  map[string]aurweb.Pkg
	sources map[string]string // pkgbase -> git clone URL
	names   []string          // sorted pkgnames, for suggest
}

func New() *Index {
	return &Index{byName: map[string]aurweb.Pkg{}, sources: map[string]string{}}
}

// Replace atomically swaps in a fresh snapshot; the sorted name list is derived
// from byName. The caller must not mutate the maps after handing them over.
func (i *Index) Replace(byName map[string]aurweb.Pkg, sources map[string]string) {
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	slices.Sort(names)

	i.mu.Lock()
	i.byName, i.sources, i.names = byName, sources, names
	i.mu.Unlock()
}

func (i *Index) Info(_ context.Context, requested []string) ([]aurweb.Pkg, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	var out []aurweb.Pkg
	for _, n := range requested {
		if p, ok := i.byName[n]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}

func (i *Index) Search(_ context.Context, by aurweb.By, arg string) ([]aurweb.Pkg, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	var out []aurweb.Pkg
	for _, p := range i.byName {
		if aurweb.Match(p, by, arg) {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(a, b aurweb.Pkg) int { return cmp.Compare(a.Name, b.Name) })
	return out, nil
}

func (i *Index) Suggest(_ context.Context, arg string, pkgbase bool) ([]string, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	pool := i.names
	if pkgbase {
		seen := map[string]bool{}
		pool = nil
		for _, p := range i.byName {
			if !seen[p.PackageBase] {
				seen[p.PackageBase] = true
				pool = append(pool, p.PackageBase)
			}
		}
		slices.Sort(pool)
	}

	var out []string
	for _, n := range pool {
		if strings.HasPrefix(n, arg) {
			out = append(out, n)
			if len(out) >= aurweb.SuggestLimit {
				break
			}
		}
	}
	return out, nil
}

func (i *Index) All(_ context.Context) ([]aurweb.Pkg, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	out := make([]aurweb.Pkg, 0, len(i.byName))
	for _, p := range i.byName {
		out = append(out, p)
	}
	return out, nil
}

func (i *Index) SourceURL(_ context.Context, pkgbase string) (string, bool, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	if u, ok := i.sources[pkgbase]; ok {
		return u, true, nil
	}
	return "", false, nil
}
