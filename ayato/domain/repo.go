package domain

import "github.com/Hayao0819/Kamisato/pkg/raiou"

type PacmanRepo struct {
	Name     string                `json:"name"`
	Arches   []string              `json:"arches"`
	Packages map[string]PacmanPkgs `json:"packages"`
}

type PacmanPkgs struct {
	Name     string          `json:"name"`
	Arch     string          `json:"arch"`
	Packages []PacmanPackage `json:"packages"`
}

// PacmanPackage is package metadata together with the canonical %FILENAME% from
// the repository database. PKGINFO intentionally does not own this field: it
// describes archive contents, while Filename belongs to repository publication.
type PacmanPackage struct {
	raiou.PKGINFO `tstype:",extends"`
	Filename      string `json:"filename"`
}
