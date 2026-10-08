package buildset

import (
	"context"
	"net/http"

	"github.com/Hayao0819/Kamisato/internal/pacman/builder"
	"github.com/Hayao0819/Kamisato/internal/pacman/host"
	"github.com/Hayao0819/Kamisato/internal/pacman/repo"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

const SchemaVersion = 1

type SourceType string

const (
	SourceLocal SourceType = "local"
	SourceAUR   SourceType = "aur"
)

type Source struct {
	Type     SourceType `json:"type"`
	Path     string     `json:"path,omitempty"`
	URL      string     `json:"url,omitempty"`
	Revision string     `json:"revision,omitempty"`
	Digest   string     `json:"digest"`
}

type Dependency struct {
	Constraint      string `json:"constraint"`
	ProviderType    string `json:"provider_type"`
	Provider        string `json:"provider"`
	ProviderPkgbase string `json:"provider_pkgbase,omitempty"`
	Repository      string `json:"repository,omitempty"`
}

type PlannedBuild struct {
	Pkgbase      string       `json:"pkgbase"`
	Version      string       `json:"version"`
	Source       Source       `json:"source"`
	Packages     []string     `json:"packages"`
	Explicit     []string     `json:"explicit"`
	Dependencies []Dependency `json:"dependencies,omitempty"`
}

type Plan struct {
	SchemaVersion int            `json:"schema_version"`
	Arch          string         `json:"arch"`
	Install       []string       `json:"install"`
	BuildOrder    []string       `json:"build_order"`
	Builds        []PlannedBuild `json:"builds"`
}

type PlanRequest struct {
	Arch         string
	Packages     []string
	LocalSources []string
	PacmanConf   string
	WorkDir      string
	KeepWork     bool
}

type BuildRequest struct {
	PlanRequest
	RepoName    string
	OutputDir   string
	Manifest    string
	LogDir      string
	Environment builder.BuildEnvironment
}

type AURClient interface {
	Info(ctx context.Context, names []string) ([]aurweb.Pkg, error)
	Search(ctx context.Context, by aurweb.By, arg string) ([]aurweb.Pkg, error)
	GitBase() string
}

type CloneSource func(ctx context.Context, url, dir string) (revision string, err error)

type RepositoryLoader func(ctx context.Context, config *host.BuildConfig) ([]repositorySnapshot, error)

type PacmanConfigLoader func(path, arch string) (*host.BuildConfig, error)

type Dependencies struct {
	HTTPClient       *http.Client
	AUR              AURClient
	Backend          builder.Backend
	Metadata         builder.MetadataBackend
	Clone            CloneSource
	LoadPacmanConfig PacmanConfigLoader
	LoadRepositories RepositoryLoader
	RepoTool         repo.Tool
}

type Service struct {
	aur              AURClient
	backend          builder.Backend
	metadata         builder.MetadataBackend
	clone            CloneSource
	loadPacmanConfig PacmanConfigLoader
	loadRepositories RepositoryLoader
	repoTool         repo.Tool
}
