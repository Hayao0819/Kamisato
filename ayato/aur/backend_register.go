package aur

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"

	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/Hayao0819/Kamisato/internal/gitcmd"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

// Register clones gitURL (at ref, if given) into a throwaway dir and persists only
// the metadata ingested from its .SRCINFO; it returns the registered pkgnames.
func (b *Backend) Register(ctx context.Context, gitURL, ref, maintainer string) (pkgbase string, names []string, err error) {
	dir, err := os.MkdirTemp("", "ayato-aur-*")
	if err != nil {
		return "", nil, errwrap.WrapErr(err, "failed to create temp clone dir")
	}
	defer func() { _ = os.RemoveAll(dir) }()

	// gitURL is admin-supplied, so Strict rejects file/ext transports and
	// private-network hosts (SSRF/RCE guard).
	if err := gitcmd.Clone(ctx, gitcmd.CloneOptions{URL: gitURL, Dir: dir, Ref: ref, Depth: 1, Strict: true}); err != nil {
		return "", nil, err
	}
	return b.ingest(ctx, dir, gitURL, maintainer)
}

func (b *Backend) ingest(ctx context.Context, dir, source, maintainer string) (pkgbase string, names []string, err error) {
	si, err := raiou.ParseSrcinfoFile(filepath.Join(dir, ".SRCINFO"))
	if err != nil {
		return "", nil, errwrap.WrapErr(err, "registered repo has no valid .SRCINFO at its root")
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
		return "", nil, errwrap.NewErr("registered repo produced no packages")
	}

	pkgbase = pkgs[0].PackageBase
	for _, p := range pkgs {
		raw, mErr := json.Marshal(p)
		if mErr != nil {
			return "", nil, errwrap.WrapErr(mErr, "failed to encode package")
		}
		if sErr := b.kv.Set(nsPkg, p.Name, raw, 0); sErr != nil {
			return "", nil, errwrap.WrapErr(sErr, "failed to store package")
		}
		names = append(names, p.Name)
	}

	rec, mErr := json.Marshal(baseRecord{URL: source, Names: names})
	if mErr != nil {
		return "", nil, errwrap.WrapErr(mErr, "failed to encode pkgbase record")
	}
	if err := b.kv.Set(nsBase, pkgbase, rec, 0); err != nil {
		return "", nil, errwrap.WrapErr(err, "failed to store pkgbase")
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
			return errwrap.WrapErr(dErr, "failed to delete package "+n)
		}
	}
	return b.kv.Delete(nsBase, pkgbase)
}

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
