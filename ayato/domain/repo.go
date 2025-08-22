package domain

import "github.com/Hayao0819/Kamisato/pkg/raiou"

// PacmanRepo represents repository information (all architectures and packages).
type PacmanRepo struct {
	Name     string                `json:"name"`
	Arches   []string              `json:"arches"`
	Packages map[string]PacmanPkgs `json:"packages"`
}

// PacmanPkgs represents a list of packages for each architecture.
type PacmanPkgs struct {
	Name     string          `json:"name"`
	Arch     string          `json:"arch"`
	Packages []raiou.PKGINFO `json:"packages"`
}
