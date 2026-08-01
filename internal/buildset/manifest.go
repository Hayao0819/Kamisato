package buildset

import "github.com/Hayao0819/Kamisato/internal/pacman/builder"

type ManifestRepository struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Database string `json:"database"`
}

type ManifestPackage struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Arch      string `json:"arch"`
	File      string `json:"file"`
	Signature string `json:"signature,omitempty"`
	SHA256    string `json:"sha256"`
	BuildInfo string `json:"build_info"`
	Explicit  bool   `json:"explicit"`
}

type ManifestBuild struct {
	Pkgbase      string            `json:"pkgbase"`
	Source       Source            `json:"source"`
	Explicit     bool              `json:"explicit"`
	Log          string            `json:"log"`
	Dependencies []Dependency      `json:"dependencies,omitempty"`
	Packages     []ManifestPackage `json:"packages"`
}

type Manifest struct {
	SchemaVersion    int                      `json:"schema_version"`
	Arch             string                   `json:"arch"`
	Backend          string                   `json:"backend"`
	BuildEnvironment builder.BuildEnvironment `json:"build_environment"`
	Repository       ManifestRepository       `json:"repository"`
	Install          []string                 `json:"install"`
	Builds           []ManifestBuild          `json:"builds"`
}
