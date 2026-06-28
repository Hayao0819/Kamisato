// Package aur is ayato's aurweb Backend. ayato is stateless, so it does not host
// git itself: an admin registers an external git URL, ayato parses that repo's
// .SRCINFO once (an ephemeral clone it immediately discards), and keeps only the
// derived metadata in the shared kv store. RPC is answered from kv; a git clone
// of a registered pkgbase is redirected to the registered URL. Anything not
// registered falls through to the real AUR.
package aur

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/internal/kayoproto"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

const (
	nsPkg  = "aurpkg"  // pkgname -> JSON(aurweb.Pkg)
	nsBase = "aurbase" // pkgbase -> JSON(baseRecord)
)

// baseRecord tracks a registered pkgbase: its git clone URL (the git-clone
// redirect target) and the pkgnames it produced, so a later removal can clean up
// every derived entry.
type baseRecord struct {
	URL   string   `json:"url"`
	Names []string `json:"names"`
}

// Backend implements aurweb.Backend over a kv.Store.
type Backend struct {
	kv           kv.Store
	defaultMaint string
}

// defaultMaintainer is the maintainer label applied to registered packages that
// do not carry their own.
func New(store kv.Store, defaultMaintainer string) *Backend {
	return &Backend{kv: store, defaultMaint: defaultMaintainer}
}

func (b *Backend) Info(_ context.Context, names []string) ([]aurweb.Pkg, error) {
	var out []aurweb.Pkg
	for _, n := range names {
		raw, err := b.kv.Get(nsPkg, n)
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
	var out []aurweb.Pkg
	for _, p := range all {
		if aurweb.Match(p, by, arg) {
			out = append(out, p)
		}
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
	names := make([]string, len(all))
	for i, p := range all {
		names[i] = p.Name
	}
	slices.Sort(names)
	return prefix(names, arg), nil
}

func (b *Backend) All(_ context.Context) ([]aurweb.Pkg, error) {
	return b.all()
}

// Catalog is the kayo-facing view: every managed package plus the git source URL
// of each pkgbase.
func (b *Backend) Catalog(_ context.Context) (kayoproto.Catalog, error) {
	pkgs, err := b.all()
	if err != nil {
		return kayoproto.Catalog{}, err
	}
	entries, err := b.kv.List(nsBase)
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
	raw, err := b.kv.Get(nsBase, pkgbase)
	if err != nil {
		return baseRecord{}, err
	}
	var rec baseRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return baseRecord{}, utils.WrapErr(err, "corrupt pkgbase record")
	}
	return rec, nil
}

func (b *Backend) all() ([]aurweb.Pkg, error) {
	entries, err := b.kv.List(nsPkg)
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
			if len(out) >= 20 {
				break
			}
		}
	}
	return out
}
