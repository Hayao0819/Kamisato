package domain

import "github.com/Hayao0819/Kamisato/raiou"

type PacmanRepo struct {
	Name     string                `json:"name"`
	Arches   []string              `json:"arches"`
	Packages map[string]PacmanPkgs `json:"packages"`
}

type PacmanPkgs struct {
	Name     string          `json:"name"`
	Arch     string          `json:"arch"`
	Packages []raiou.PKGINFO `json:"packages"`
}
