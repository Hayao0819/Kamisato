package domain

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

// PackageMetadata is the stable package representation exposed by Ayato's API.
// It is deliberately independent of the parser and repository implementations:
// those packages may evolve their internal models without changing this wire
// contract.
type PackageMetadata struct {
	PkgName     string            `json:"pkgname"`
	PkgBase     string            `json:"pkgbase"`
	PkgVer      string            `json:"pkgver"`
	PkgDesc     string            `json:"pkgdesc"`
	URL         string            `json:"url"`
	BuildDate   int64             `json:"builddate"`
	Packager    string            `json:"packager"`
	Size        int64             `json:"size"`
	Arch        string            `json:"arch"`
	License     []string          `json:"license"`
	Replaces    []string          `json:"replaces"`
	Group       []string          `json:"group"`
	Conflict    []string          `json:"conflict"`
	Provides    []string          `json:"provides"`
	Backup      []string          `json:"backup"`
	Depend      []string          `json:"depend"`
	OptDepend   []string          `json:"optdepend"`
	MakeDepend  []string          `json:"makedepend"`
	CheckDepend []string          `json:"checkdepend"`
	XData       map[string]string `json:"xdata"`
	PkgType     string            `json:"pkgtype"`
}

// PacmanPackage is package metadata together with the canonical %FILENAME% from
// the repository database. PackageMetadata intentionally does not own this
// field: it describes archive contents, while Filename belongs to repository
// publication.
type PacmanPackage struct {
	PackageMetadata `tstype:",extends"`
	Filename        string `json:"filename"`
}
