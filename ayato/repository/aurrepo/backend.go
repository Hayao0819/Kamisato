// Package aurrepo is ayato's stateless aurweb backend: an admin registers an external
// git URL, ayato parses its .SRCINFO from an ephemeral clone it discards, and keeps
// only the derived metadata in kv. A registered pkgbase's git clone is redirected to
// the registered URL; anything unregistered falls through to the real AUR.
package aurrepo

import (
	"cmp"
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/samber/lo"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/internal/kayoproto"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

// baseRecord stores a registered pkgbase's clone URL and the pkgnames it produced,
// so removal can clean up every derived entry.
type baseRecord struct {
	URL   string   `json:"url"`
	Names []string `json:"names"`
}

type Backend struct {
	kv           kv.Store
	defaultMaint string
}

func NewBackend(store kv.Store, defaultMaintainer string) *Backend {
	return &Backend{kv: store, defaultMaint: defaultMaintainer}
}

func (b *Backend) Info(_ context.Context, names []string) ([]aurweb.Pkg, error) {
	var out []aurweb.Pkg
	for _, n := range names {
		raw, err := b.kv.Get(kv.AURPackages, n)
		if errors.Is(err, kv.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var p aurweb.Pkg
		if json.Unmarshal(raw, &p) == nil {
			out = append(out, p)
		}
	}
	return out, nil
}

func (b *Backend) Search(_ context.Context, by aurweb.By, arg string) ([]aurweb.Pkg, error) {
	all, err := b.all()
	if err != nil {
		return nil, err
	}
	out := lo.Filter(all, func(pkg aurweb.Pkg, _ int) bool {
		return aurweb.Match(pkg, by, arg)
	})
	if len(out) == 0 {
		return nil, nil
	}
	slices.SortFunc(out, func(a, b aurweb.Pkg) int { return cmp.Compare(a.Name, b.Name) })
	return out, nil
}

func (b *Backend) Suggest(ctx context.Context, arg string, pkgbase bool) ([]string, error) {
	if pkgbase {
		bases, err := b.List(ctx)
		if err != nil {
			return nil, err
		}
		return prefix(bases, arg), nil
	}
	all, err := b.all()
	if err != nil {
		return nil, err
	}
	names := lo.Map(all, func(pkg aurweb.Pkg, _ int) string { return pkg.Name })
	slices.Sort(names)
	return prefix(names, arg), nil
}

func (b *Backend) All(_ context.Context) ([]aurweb.Pkg, error) {
	return b.all()
}

func (b *Backend) Catalog(_ context.Context) (kayoproto.Catalog, error) {
	pkgs, err := b.all()
	if err != nil {
		return kayoproto.Catalog{}, err
	}
	entries, err := b.kv.List(kv.AURBases)
	if err != nil {
		return kayoproto.Catalog{}, err
	}
	sources := make(map[string]string, len(entries))
	for _, e := range entries {
		var rec baseRecord
		if json.Unmarshal(e.Value, &rec) == nil {
			sources[e.Key] = rec.URL
		}
	}
	return kayoproto.Catalog{Packages: pkgs, Sources: sources}, nil
}

func (b *Backend) SourceURL(_ context.Context, pkgbase string) (string, bool, error) {
	rec, err := b.base(pkgbase)
	if errors.Is(err, kv.ErrNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return rec.URL, true, nil
}

func (b *Backend) base(pkgbase string) (baseRecord, error) {
	raw, err := b.kv.Get(kv.AURBases, pkgbase)
	if err != nil {
		return baseRecord{}, err
	}
	var rec baseRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return baseRecord{}, errors.WrapErr(err, "corrupt pkgbase record")
	}
	return rec, nil
}

func (b *Backend) all() ([]aurweb.Pkg, error) {
	entries, err := b.kv.List(kv.AURPackages)
	if err != nil {
		return nil, err
	}
	out := make([]aurweb.Pkg, 0, len(entries))
	for _, e := range entries {
		var p aurweb.Pkg
		if json.Unmarshal(e.Value, &p) == nil {
			out = append(out, p)
		}
	}
	return out, nil
}

func prefix(pool []string, arg string) []string {
	var out []string
	for _, n := range pool {
		if strings.HasPrefix(n, arg) {
			out = append(out, n)
			if len(out) >= aurweb.SuggestLimit {
				break
			}
		}
	}
	return out
}
