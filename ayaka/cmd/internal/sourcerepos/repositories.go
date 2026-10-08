// Package sourcerepos prepares lazy source repository input for CLI commands.
package sourcerepos

import (
	"fmt"
	"slices"
	"sync"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/settings"
	"github.com/Hayao0819/Kamisato/ayaka/config"
	"github.com/Hayao0819/Kamisato/ayaka/source"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/spf13/cobra"
)

type ListFunc func() ([]*source.SourceRepo, error)
type LookupFunc func(string) (*source.SourceRepo, error)

// Reader is limited to repository discovery. Commands must receive other
// dependencies separately rather than reaching through a global App object.
type Reader interface {
	All() ([]*source.SourceRepo, error)
	Find(string) (*source.SourceRepo, error)
}

type repositories ListFunc

// ForCommand defers source input until execution or completion needs it.
func ForCommand(command *cobra.Command) Reader {
	return New(func() ([]*source.SourceRepo, error) {
		cfg, err := settings.Load(command)
		if err != nil {
			return nil, err
		}
		return Load(cfg.Repos)
	})
}

// New defers loading until the first lookup, including shell completion.
// sync.OnceValues caches both successful results and failures concurrently.
func New(load ListFunc) Reader {
	return repositories(sync.OnceValues(func() ([]*source.SourceRepo, error) {
		if load == nil {
			return nil, fmt.Errorf("source repository loader is not configured")
		}
		repos, err := load()
		return slices.Clone(repos), err
	}))
}

func Static(repos []*source.SourceRepo) Reader {
	snapshot := slices.Clone(repos)
	return New(func() ([]*source.SourceRepo, error) { return snapshot, nil })
}

func (read repositories) All() ([]*source.SourceRepo, error) {
	repos, err := read()
	return slices.Clone(repos), err
}

// Find allows a missing repository (remote git builds do not require one).
func (read repositories) Find(name string) (*source.SourceRepo, error) {
	repos, err := read()
	if err != nil {
		return nil, err
	}
	for _, repo := range repos {
		if repo != nil && repo.Config != nil && repo.Config.Name == name {
			return repo, nil
		}
	}
	return nil, nil
}

// Require turns a discovery function into a mandatory repository lookup.
func Require(find LookupFunc, name string) (*source.SourceRepo, error) {
	repo, err := find(name)
	if err != nil {
		return nil, err
	}
	if repo == nil {
		return nil, errors.WrapErr(ErrSourceRepoNotFound, name)
	}
	return repo, nil
}

// Select implements the optional single-repository argument used by reports.
func Select(reader Reader, names []string) ([]*source.SourceRepo, error) {
	if len(names) == 0 {
		return reader.All()
	}
	repo, err := Require(reader.Find, names[0])
	if err != nil {
		return nil, err
	}
	return []*source.SourceRepo{repo}, nil
}

// Load assembles only repository dependencies from explicitly supplied entries.
func Load(entries []config.RepoEntry) ([]*source.SourceRepo, error) {
	repos := make([]*source.SourceRepo, 0, len(entries))
	for _, entry := range entries {
		cfg, err := source.LoadConfig(entry.Dir)
		if err != nil {
			return nil, errors.WrapErr(err, "failed to load source repository config "+entry.Dir)
		}
		if cfg.Build.Makepkg.Packager == "" {
			cfg.Build.Makepkg.Packager = cfg.Maintainer
		}
		repo, err := source.GetSrcRepo(entry.Dir, cfg)
		if err != nil {
			return nil, errors.WrapErr(err, "failed to load source repository "+entry.Dir)
		}
		repo.Dir, repo.DestDir = entry.Dir, entry.DestDir
		repos = append(repos, repo)
	}
	return repos, nil
}
