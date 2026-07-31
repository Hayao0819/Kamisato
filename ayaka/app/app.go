// Package app is ayaka's per-invocation composition root: the loaded config
// plus the source repositories it declares.
package app

import (
	"sync"

	"github.com/samber/lo"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/pacman/source"
)

// App is the dependency set shared by one Ayaka command invocation.
type App struct {
	Config   *AyakaConfig
	SrcRepos []*source.SourceRepo
}

// New loads the source repositories declared in cfg and returns the App.
func New(cfg *AyakaConfig) (*App, error) {
	app := &App{Config: cfg}
	for _, r := range cfg.Repos {
		repoconfig, err := source.LoadConfig(r.Dir)
		if err != nil {
			return nil, errors.WrapErr(err, "failed to load source repository config "+r.Dir)
		}
		if repoconfig.Build.Makepkg.Packager == "" {
			repoconfig.Build.Makepkg.Packager = repoconfig.Maintainer
		}
		sr, err := source.GetSrcRepo(r.Dir, repoconfig)
		if err != nil {
			return nil, errors.WrapErr(err, "failed to load source repository "+r.Dir)
		}
		sr.Dir = r.Dir
		sr.DestDir = r.DestDir
		app.SrcRepos = append(app.SrcRepos, sr)
	}
	return app, nil
}

type Runtime struct {
	load        func() (*App, error)
	once        sync.Once
	mu          sync.RWMutex
	app         *App
	err         error
	initialized bool
}

func NewRuntime(load func() (*App, error)) *Runtime {
	return &Runtime{load: load}
}

func StaticRuntime(a *App) *Runtime {
	return NewRuntime(func() (*App, error) { return a, nil })
}

func (r *Runtime) App() (*App, error) {
	r.once.Do(func() {
		var loaded *App
		var loadErr error
		if r.load == nil {
			loadErr = errors.New("ayaka application loader is not configured")
		} else {
			loaded, loadErr = r.load()
			if loadErr == nil && loaded == nil {
				loadErr = errors.New("ayaka application loader returned nil")
			}
		}
		r.mu.Lock()
		r.app, r.err, r.initialized = loaded, loadErr, true
		r.mu.Unlock()
	})
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.app, r.err
}

func (r *Runtime) LoadedApp() (*App, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.app, r.initialized && r.err == nil
}

func (a *App) GetSrcRepo(name string) *source.SourceRepo {
	for _, r := range a.SrcRepos {
		if r.Config.Name == name {
			return r
		}
	}
	return nil
}

func (a *App) GetSrcRepoNames() []string {
	return lo.Map(a.SrcRepos, func(r *source.SourceRepo, _ int) string {
		return r.Config.Name
	})
}
