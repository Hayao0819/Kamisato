// Package aurweb implements an aurweb (AUR) v5-compatible RPC and source
// interface that any host can serve. A host supplies a Backend (its own package
// set) and an optional Upstream (the real AUR) for fallback; the Server turns
// those into the HTTP surface that yay, paru and other AUR helpers speak to.
//
// The package is framework-neutral: it depends only on net/http, so it mounts
// equally well in a plain mux (kayo) or behind gin (ayato).
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

// searchResult is the search-level record: the fields every result carries.
// Nullable columns are pointers so an empty value renders as JSON null, not "".
type searchResult struct {
	ID             int     `json:"ID"`
	Name           string  `json:"Name"`
	PackageBaseID  int     `json:"PackageBaseID"`
	PackageBase    string  `json:"PackageBase"`
	Version        string  `json:"Version"`
	Description    *string `json:"Description"`
	URL            *string `json:"URL"`
	NumVotes       int     `json:"NumVotes"`
	Popularity     float64 `json:"Popularity"`
	OutOfDate      *int    `json:"OutOfDate"`
	Maintainer     *string `json:"Maintainer"`
	FirstSubmitted int64   `json:"FirstSubmitted"`
	LastModified   int64   `json:"LastModified"`
	URLPath        string  `json:"URLPath"`
}

// infoResult is the info-level record: the search fields plus Submitter, the
// always-present License/Keywords arrays, and the relation arrays that aurweb
// emits only when non-empty.
type infoResult struct {
	searchResult
	Submitter     *string  `json:"Submitter"`
	License       []string `json:"License"`
	Keywords      []string `json:"Keywords"`
	Depends       []string `json:"Depends,omitempty"`
	MakeDepends   []string `json:"MakeDepends,omitempty"`
	CheckDepends  []string `json:"CheckDepends,omitempty"`
	OptDepends    []string `json:"OptDepends,omitempty"`
	Conflicts     []string `json:"Conflicts,omitempty"`
	Provides      []string `json:"Provides,omitempty"`
	Replaces      []string `json:"Replaces,omitempty"`
	Groups        []string `json:"Groups,omitempty"`
	CoMaintainers []string `json:"CoMaintainers,omitempty"`
}

func (p Pkg) base() searchResult {
	return searchResult{
		ID:             p.ID,
		Name:           p.Name,
		PackageBaseID:  p.PackageBaseID,
		PackageBase:    p.PackageBase,
		Version:        p.Version,
		Description:    nullStr(p.Description),
		URL:            nullStr(p.URL),
		NumVotes:       p.NumVotes,
		Popularity:     p.Popularity,
		OutOfDate:      outOfDate(p.OutOfDate),
		Maintainer:     nullStr(p.Maintainer),
		FirstSubmitted: p.FirstSubmitted,
		LastModified:   p.LastModified,
		URLPath:        p.URLPath,
	}
}

// result renders the record as aurweb does: search level returns the bare base,
// info level adds Submitter, License/Keywords and the non-empty relation arrays.
func (p Pkg) result(info bool) any {
	if !info {
		return p.base()
	}
	return infoResult{
		searchResult:  p.base(),
		Submitter:     nullStr(p.Submitter),
		License:       nonNilSlice(p.License),
		Keywords:      nonNilSlice(p.Keywords),
		Depends:       p.Depends,
		MakeDepends:   p.MakeDepends,
		CheckDepends:  p.CheckDepends,
		OptDepends:    p.OptDepends,
		Conflicts:     p.Conflicts,
		Provides:      p.Provides,
		Replaces:      p.Replaces,
		Groups:        p.Groups,
		CoMaintainers: p.CoMaintainers,
	}
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func outOfDate(ts int) *int {
	if ts > 0 {
		return &ts
	}
	return nil
}

func nonNilSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
