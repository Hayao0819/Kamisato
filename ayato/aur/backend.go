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
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/internal/gitcmd"
	"github.com/Hayao0819/Kamisato/internal/kayoproto"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
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

// New builds a Backend. defaultMaintainer is the maintainer label applied to
// registered packages that do not carry their own.
func New(store kv.Store, defaultMaintainer string) *Backend {
	return &Backend{kv: store, defaultMaint: defaultMaintainer}
}

// Register clones gitURL (at ref, if given) into a throwaway temp dir, parses
// its .SRCINFO, and persists the derived aurweb records plus the pkgbase->URL
// mapping. The clone is deleted before returning; only metadata survives. It
// returns the pkgnames it registered.
func (b *Backend) Register(ctx context.Context, gitURL, ref, maintainer string) (pkgbase string, names []string, err error) {
	dir, err := os.MkdirTemp("", "ayato-aur-*")
	if err != nil {
		return "", nil, utils.WrapErr(err, "failed to create temp clone dir")
	}
	defer func() { _ = os.RemoveAll(dir) }()

	// Strict: gitURL comes from an admin request, so validate it and refuse
	// file/ext transports and private-network hosts (SSRF/RCE guard).
	if err := gitcmd.Clone(ctx, gitcmd.CloneOptions{URL: gitURL, Dir: dir, Ref: ref, Depth: 1, Strict: true}); err != nil {
		return "", nil, err
	}
	return b.ingest(ctx, dir, gitURL, maintainer)
}

// ingest parses the .SRCINFO in a checked-out dir and persists the derived
// records, recording source as the pkgbase's clone URL.
func (b *Backend) ingest(ctx context.Context, dir, source, maintainer string) (pkgbase string, names []string, err error) {
	si, err := raiou.ParseSrcinfoFile(filepath.Join(dir, ".SRCINFO"))
	if err != nil {
		return "", nil, utils.WrapErr(err, "registered repo has no valid .SRCINFO at its root")
	}

	if maintainer == "" {
		maintainer = b.defaultMaint
	}
	ts := gitcmd.CommitUnix(ctx, dir)
	pkgs := aurweb.FromSrcinfo(si, aurweb.SrcinfoMeta{
		Maintainer:     maintainer,
		Submitter:      maintainer,
		FirstSubmitted: ts,
		LastModified:   ts,
	})
	if len(pkgs) == 0 {
		return "", nil, utils.NewErr("registered repo produced no packages")
	}

	pkgbase = pkgs[0].PackageBase
	for _, p := range pkgs {
		raw, mErr := json.Marshal(p)
		if mErr != nil {
			return "", nil, utils.WrapErr(mErr, "failed to encode package")
		}
		if sErr := b.kv.Set(nsPkg, p.Name, raw, 0); sErr != nil {
			return "", nil, utils.WrapErr(sErr, "failed to store package")
		}
		names = append(names, p.Name)
	}

	rec, _ := json.Marshal(baseRecord{URL: source, Names: names})
	if err := b.kv.Set(nsBase, pkgbase, rec, 0); err != nil {
		return "", nil, utils.WrapErr(err, "failed to store pkgbase")
	}
	return pkgbase, names, nil
}

// Remove deletes a registered pkgbase and every package it produced.
func (b *Backend) Remove(_ context.Context, pkgbase string) error {
	rec, err := b.base(pkgbase)
	if err != nil {
		return err
	}
	for _, n := range rec.Names {
		if dErr := b.kv.Delete(nsPkg, n); dErr != nil {
			return utils.WrapErr(dErr, "failed to delete package "+n)
		}
	}
	return b.kv.Delete(nsBase, pkgbase)
}

// List returns the registered pkgbases.
func (b *Backend) List(_ context.Context) ([]string, error) {
	entries, err := b.kv.List(nsBase)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Key
	}
	slices.Sort(out)
	return out, nil
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

func (b *Backend) Suggest(_ context.Context, arg string, pkgbase bool) ([]string, error) {
	if pkgbase {
		bases, err := b.List(context.Background())
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
