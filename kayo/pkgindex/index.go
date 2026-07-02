// Package pkgindex is a concurrency-safe in-memory aurweb index: a pkgname->record
// map with prefix suggest and a pkgbase->clone-URL side table. overlay.Registry and
// ayato.Source both embed it so the read-side Backend methods (Info/Search/Suggest/
// All/SourceURL) live in one place and cannot drift apart.
//
// Reads serve entirely from an immutable snapshot published by a single atomic
// pointer swap, so a periodic refresh never blocks in-flight readers and a reader
// never observes a half-built index (the goaurrpc pattern).
package pkgindex

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

// snapshot is one immutable view of the index. A reader loads the current pointer
// once and reads the maps lock-free; nothing mutates a published snapshot, so the
// only synchronization is the atomic swap in Replace.
type snapshot struct {
	byName  map[string]aurweb.Pkg
	sources map[string]string // pkgbase -> git clone URL
	names   []string          // sorted pkgnames, for suggest
}

// Index holds the current snapshot behind an atomic pointer. The zero value is not
// usable; call New.
type Index struct {
	cur atomic.Pointer[snapshot]
}

func New() *Index {
	i := &Index{}
	i.cur.Store(&snapshot{byName: map[string]aurweb.Pkg{}, sources: map[string]string{}})
	return i
}

// Replace atomically swaps in a fresh snapshot; the sorted name list is derived
// from byName. The caller must not mutate the maps after handing them over.
func (i *Index) Replace(byName map[string]aurweb.Pkg, sources map[string]string) {
	if byName == nil {
		byName = map[string]aurweb.Pkg{}
	}
	if sources == nil {
		sources = map[string]string{}
	}
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	slices.Sort(names)
	i.cur.Store(&snapshot{byName: byName, sources: sources, names: names})
}

func (i *Index) Info(_ context.Context, requested []string) ([]aurweb.Pkg, error) {
	s := i.cur.Load()
	var out []aurweb.Pkg
	for _, n := range requested {
		if p, ok := s.byName[n]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}

func (i *Index) Search(_ context.Context, by aurweb.By, arg string) ([]aurweb.Pkg, error) {
	s := i.cur.Load()
	var out []aurweb.Pkg
	for _, p := range s.byName {
		if aurweb.Match(p, by, arg) {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(a, b aurweb.Pkg) int { return cmp.Compare(a.Name, b.Name) })
	return out, nil
}

func (i *Index) Suggest(_ context.Context, arg string, pkgbase bool) ([]string, error) {
	s := i.cur.Load()

	pool := s.names
	if pkgbase {
		seen := map[string]bool{}
		pool = nil
		for _, p := range s.byName {
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
	s := i.cur.Load()
	out := make([]aurweb.Pkg, 0, len(s.byName))
	for _, p := range s.byName {
		out = append(out, p)
	}
	return out, nil
}

func (i *Index) SourceURL(_ context.Context, pkgbase string) (string, bool, error) {
	s := i.cur.Load()
	if u, ok := s.sources[pkgbase]; ok {
		return u, true, nil
	}
	return "", false, nil
}
