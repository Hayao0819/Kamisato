package ayato

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

func (s *Source) Info(_ context.Context, requested []string) ([]aurweb.Pkg, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []aurweb.Pkg
	for _, n := range requested {
		if p, ok := s.index[n]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *Source) Search(_ context.Context, by aurweb.By, arg string) ([]aurweb.Pkg, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []aurweb.Pkg
	for _, p := range s.index {
		if aurweb.Match(p, by, arg) {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(a, b aurweb.Pkg) int { return cmp.Compare(a.Name, b.Name) })
	return out, nil
}

func (s *Source) Suggest(_ context.Context, arg string, pkgbase bool) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pool := s.names
	if pkgbase {
		seen := map[string]bool{}
		pool = nil
		for _, p := range s.index {
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

func (s *Source) All(_ context.Context) ([]aurweb.Pkg, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]aurweb.Pkg, 0, len(s.index))
	for _, p := range s.index {
		out = append(out, p)
	}
	return out, nil
}

func (s *Source) SourceURL(_ context.Context, pkgbase string) (string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if u, ok := s.sources[pkgbase]; ok {
		return u, true, nil
	}
	return "", false, nil
}
