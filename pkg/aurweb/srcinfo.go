package aurweb

import (
	"cmp"
	"hash/fnv"
	"slices"

	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

// SrcinfoMeta carries the account-level facts a .SRCINFO cannot provide. A
// private instance typically sets a fixed Maintainer and uses the source's last
// commit time for the timestamps.
type SrcinfoMeta struct {
	Maintainer     string
	Submitter      string
	FirstSubmitted int64
	LastModified   int64
	// URLPath, when set, overrides the default snapshot path. Most hosts serve
	// git via redirect and can leave this empty.
	URLPath string
}

// FromSrcinfo turns a parsed .SRCINFO into one Pkg per split package, merging the
// pkgbase-level (global) relations with each package's own across all
// architectures so the result matches what aurweb emits for that pkgname.
func FromSrcinfo(si *raiou.SRCINFO, meta SrcinfoMeta) []Pkg {
	if si == nil {
		return nil
	}

	base := si.SrcinfoBase
	global := si.SrcinfoPackage
	version := joinVersion(base.PkgVer, base.PkgRel, base.PkgEpoch)
	baseID := stableID(base.PkgBase)
	urlPath := meta.URLPath
	if urlPath == "" {
		urlPath = "/cgit/aur.git/snapshot/" + base.PkgBase + ".tar.gz"
	}

	pkgs := si.Packages
	if len(pkgs) == 0 {
		pkgs = []raiou.SrcinfoPackage{global}
	}

	out := make([]Pkg, 0, len(pkgs))
	for _, p := range pkgs {
		desc := cmp.Or(p.PkgDesc, global.PkgDesc)
		url := cmp.Or(p.URL, global.URL)
		out = append(out, Pkg{
			ID:             stableID(p.PkgName),
			Name:           p.PkgName,
			PackageBaseID:  baseID,
			PackageBase:    base.PkgBase,
			Version:        version,
			Description:    desc,
			URL:            url,
			Maintainer:     meta.Maintainer,
			Submitter:      meta.Submitter,
			FirstSubmitted: meta.FirstSubmitted,
			LastModified:   meta.LastModified,
			URLPath:        urlPath,

			Depends:      mergeArch(global.Depends, p.Depends),
			OptDepends:   mergeArch(global.OptDepends, p.OptDepends),
			Provides:     mergeArch(global.Provides, p.Provides),
			Conflicts:    mergeArch(global.Conflicts, p.Conflicts),
			Replaces:     mergeArch(global.Replaces, p.Replaces),
			MakeDepends:  mergeArch(base.MakeDepends, nil),
			CheckDepends: mergeArch(base.CheckDepends, nil),
			Groups:       mergeList(global.Groups, p.Groups),
			License:      mergeList(global.License, p.License),
		})
	}
	return out
}

// joinVersion renders the aurweb Version string: "[epoch:]pkgver-pkgrel".
func joinVersion(ver, rel, epoch string) string {
	v := ver
	if rel != "" {
		v += "-" + rel
	}
	if epoch != "" {
		v = epoch + ":" + v
	}
	return v
}

// mergeArch unions two per-arch maps across all architectures, sorted.
func mergeArch(global, pkg raiou.ArchStrings) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range []raiou.ArchStrings{global, pkg} {
		for _, vals := range m {
			for _, v := range vals {
				if v == "" || seen[v] {
					continue
				}
				seen[v] = true
				out = append(out, v)
			}
		}
	}
	slices.Sort(out)
	return out
}

func mergeList(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, list := range [][]string{a, b} {
		for _, v := range list {
			if v == "" || seen[v] {
				continue
			}
			seen[v] = true
			out = append(out, v)
		}
	}
	slices.Sort(out)
	return out
}

// stableID derives a deterministic positive int from a name; aurweb IDs are
// opaque to helpers, so a hash avoids a global counter.
func stableID(name string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return int(h.Sum32() & 0x7fffffff)
}
