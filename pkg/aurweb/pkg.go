// Package aurweb implements an aurweb (AUR) v5-compatible RPC and source
// interface that any host can serve. A host supplies a Backend (its own package
// set) and an optional Upstream (the real AUR) for fallback; the Server turns
// those into the HTTP surface that yay, paru and other AUR helpers speak to.
//
// The package is framework-neutral: it depends only on net/http, so it mounts
// equally well in a plain mux (sara) or behind gin (ayato).
package aurweb

// Version is the only aurweb RPC version this package exposes. aurweb itself
// exposes nothing else; v6 is an unimplemented upstream TODO.
const Version = 5

// Pkg is a single aurweb v5 result record. Backends populate it from their own
// metadata; the Server shapes it into the info- or search-level JSON object
// that aurweb emits. Relation entries (Depends, Provides, ...) carry their
// version constraint inline, e.g. "glibc>=2.34".
type Pkg struct {
	ID             int
	Name           string
	PackageBaseID  int
	PackageBase    string
	Version        string
	Description    string
	URL            string
	NumVotes       int
	Popularity     float64
	OutOfDate      int    // unix ts the package was flagged out-of-date; 0 = not flagged
	Maintainer     string // "" renders as null (orphan)
	Submitter      string
	FirstSubmitted int64
	LastModified   int64
	URLPath        string

	Depends       []string
	MakeDepends   []string
	CheckDepends  []string
	OptDepends    []string
	Conflicts     []string
	Provides      []string
	Replaces      []string
	Groups        []string
	License       []string
	Keywords      []string
	CoMaintainers []string
}

// toMap renders the record as aurweb does: Submitter and the relation arrays are
// info-only, and License/Keywords are always present for info even when empty.
func (p Pkg) toMap(info bool) map[string]any {
	m := map[string]any{
		"ID":             p.ID,
		"Name":           p.Name,
		"PackageBaseID":  p.PackageBaseID,
		"PackageBase":    p.PackageBase,
		"Version":        p.Version,
		"Description":    nullableStr(p.Description),
		"URL":            nullableStr(p.URL),
		"NumVotes":       p.NumVotes,
		"Popularity":     p.Popularity,
		"OutOfDate":      nil,
		"Maintainer":     nullableStr(p.Maintainer),
		"FirstSubmitted": p.FirstSubmitted,
		"LastModified":   p.LastModified,
		"URLPath":        p.URLPath,
	}
	if p.OutOfDate > 0 {
		m["OutOfDate"] = p.OutOfDate
	}
	if !info {
		return m
	}

	m["Submitter"] = nullableStr(p.Submitter)
	m["License"] = nonNilSlice(p.License)
	m["Keywords"] = nonNilSlice(p.Keywords)
	for key, vals := range map[string][]string{
		"Depends":       p.Depends,
		"MakeDepends":   p.MakeDepends,
		"CheckDepends":  p.CheckDepends,
		"OptDepends":    p.OptDepends,
		"Conflicts":     p.Conflicts,
		"Provides":      p.Provides,
		"Replaces":      p.Replaces,
		"Groups":        p.Groups,
		"CoMaintainers": p.CoMaintainers,
	} {
		if len(vals) > 0 {
			m[key] = vals
		}
	}
	return m
}

func nullableStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nonNilSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
