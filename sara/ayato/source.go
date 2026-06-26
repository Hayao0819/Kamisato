// Package ayato makes a remote ayato instance act as a sara package source. It
// fetches the instance's catalog (its own-hosted PKGBUILDs plus their git URLs)
// and implements aurweb.Backend so sara can federate ayato alongside local git
// overlays and the upstream AUR.
package ayato

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Hayao0819/Kamisato/internal/saraproto"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

const catalogPath = "/api/unstable/aur/catalog"

// Source is one ayato instance, refreshed by Sync.
type Source struct {
	name   string
	base   string
	client *http.Client

	mu      sync.RWMutex
	index   map[string]aurweb.Pkg
	sources map[string]string
	names   []string
}

func New(name, base string) *Source {
	return &Source{
		name:    name,
		base:    strings.TrimRight(base, "/"),
		client:  &http.Client{Timeout: 15 * time.Second},
		index:   map[string]aurweb.Pkg{},
		sources: map[string]string{},
	}
}

// Sync fetches the instance catalog and swaps in a fresh index.
func (s *Source) Sync(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base+catalogPath, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return utils.WrapErr(err, "ayato catalog request: "+s.name)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return utils.NewErrf("ayato %s: catalog status %d", s.name, resp.StatusCode)
	}

	var cat saraproto.Catalog
	if err := json.NewDecoder(resp.Body).Decode(&cat); err != nil {
		return utils.WrapErr(err, "ayato catalog decode: "+s.name)
	}

	index := make(map[string]aurweb.Pkg, len(cat.Packages))
	names := make([]string, 0, len(cat.Packages))
	for _, p := range cat.Packages {
		index[p.Name] = p
		names = append(names, p.Name)
	}
	sort.Strings(names)

	s.mu.Lock()
	s.index, s.sources, s.names = index, cat.Sources, names
	s.mu.Unlock()
	return nil
}

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
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
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
		sort.Strings(pool)
	}

	var out []string
	for _, n := range pool {
		if strings.HasPrefix(n, arg) {
			out = append(out, n)
			if len(out) >= 20 {
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
