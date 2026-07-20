package aurweb

import (
	"hash/fnv"

	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

// SrcinfoMeta carries account-level facts a .SRCINFO cannot provide (maintainer, submitter, timestamps).
type SrcinfoMeta struct {
	Maintainer     string
	Submitter      string
	FirstSubmitted int64
	LastModified   int64
	// URLPath, when set, overrides the default snapshot path. Most hosts serve
	// git via redirect and can leave this empty.
	URLPath string
}

// FromSrcinfo converts a parsed .SRCINFO into one Pkg per resolved split
// package, flattening architecture groups to match aurweb's output.
func FromSrcinfo(si *raiou.SRCINFO, meta SrcinfoMeta) []Pkg {
	if si == nil {
		return nil
	}

	base := si.SrcinfoBase
	global := si.SrcinfoPackage
	version := base.Version()
	baseID := stableID(base.PkgBase)
	urlPath := meta.URLPath
	if urlPath == "" {
		urlPath = "/cgit/aur.git/snapshot/" + base.PkgBase + ".tar.gz"
	}

	pkgs := si.SplitPackages()
	if len(pkgs) == 0 {
		pkgs = []raiou.SrcinfoPackage{global}
	}

	out := make([]Pkg, 0, len(pkgs))
	for _, p := range pkgs {
		out = append(out, Pkg{
			ID:             stableID(p.PkgName),
			Name:           p.PkgName,
			PackageBaseID:  baseID,
			PackageBase:    base.PkgBase,
			Version:        version,
			Description:    p.PkgDesc,
			URL:            p.URL,
			Maintainer:     meta.Maintainer,
			Submitter:      meta.Submitter,
			FirstSubmitted: meta.FirstSubmitted,
			LastModified:   meta.LastModified,
			URLPath:        urlPath,

			Depends:      sortedNonEmpty(p.Depends.All()),
			OptDepends:   sortedNonEmpty(p.OptDepends.All()),
			Provides:     sortedNonEmpty(p.Provides.All()),
			Conflicts:    sortedNonEmpty(p.Conflicts.All()),
			Replaces:     sortedNonEmpty(p.Replaces.All()),
			MakeDepends:  sortedNonEmpty(base.MakeDepends.All()),
			CheckDepends: sortedNonEmpty(base.CheckDepends.All()),
			Groups:       sortedNonEmpty(p.Groups),
			License:      sortedNonEmpty(p.License),
		})
	}
	return out
}

// stableID derives a deterministic positive int from a name; aurweb IDs are
// opaque to helpers, so a hash avoids a global counter.
func stableID(name string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return int(h.Sum32() & 0x7fffffff)
}
