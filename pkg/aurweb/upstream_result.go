package aurweb

// rpcResult mirrors the aurweb v5 result schema with nullable fields modelled as
// pointers so JSON null decodes cleanly.
type rpcResult struct {
	ID             int      `json:"ID"`
	Name           string   `json:"Name"`
	PackageBaseID  int      `json:"PackageBaseID"`
	PackageBase    string   `json:"PackageBase"`
	Version        string   `json:"Version"`
	Description    *string  `json:"Description"`
	URL            *string  `json:"URL"`
	NumVotes       int      `json:"NumVotes"`
	Popularity     float64  `json:"Popularity"`
	OutOfDate      *int     `json:"OutOfDate"`
	Maintainer     *string  `json:"Maintainer"`
	Submitter      *string  `json:"Submitter"`
	FirstSubmitted int64    `json:"FirstSubmitted"`
	LastModified   int64    `json:"LastModified"`
	URLPath        string   `json:"URLPath"`
	Depends        []string `json:"Depends"`
	MakeDepends    []string `json:"MakeDepends"`
	CheckDepends   []string `json:"CheckDepends"`
	OptDepends     []string `json:"OptDepends"`
	Conflicts      []string `json:"Conflicts"`
	Provides       []string `json:"Provides"`
	Replaces       []string `json:"Replaces"`
	Groups         []string `json:"Groups"`
	License        []string `json:"License"`
	Keywords       []string `json:"Keywords"`
	CoMaintainers  []string `json:"CoMaintainers"`
}

func (r rpcResult) toPkg() Pkg {
	p := Pkg{
		ID:             r.ID,
		Name:           r.Name,
		PackageBaseID:  r.PackageBaseID,
		PackageBase:    r.PackageBase,
		Version:        r.Version,
		Description:    derefStr(r.Description),
		URL:            derefStr(r.URL),
		NumVotes:       r.NumVotes,
		Popularity:     r.Popularity,
		Maintainer:     derefStr(r.Maintainer),
		Submitter:      derefStr(r.Submitter),
		FirstSubmitted: r.FirstSubmitted,
		LastModified:   r.LastModified,
		URLPath:        r.URLPath,
		Depends:        r.Depends,
		MakeDepends:    r.MakeDepends,
		CheckDepends:   r.CheckDepends,
		OptDepends:     r.OptDepends,
		Conflicts:      r.Conflicts,
		Provides:       r.Provides,
		Replaces:       r.Replaces,
		Groups:         r.Groups,
		License:        r.License,
		Keywords:       r.Keywords,
		CoMaintainers:  r.CoMaintainers,
	}
	if r.OutOfDate != nil {
		p.OutOfDate = *r.OutOfDate
	}
	return p
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
