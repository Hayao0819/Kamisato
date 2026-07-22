// Package app is ayaka's per-invocation composition root: the loaded config
// plus the source repositories it declares, threaded through the command
// context so the cobra layer stays wiring-only.
package app

import (
	"context"

	"github.com/samber/lo"
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// App is the per-invocation dependency set threaded via the command context,
// so commands read deps from here instead of mutable package globals.
type App struct {
	Config   *conf.AyakaConfig
	SrcRepos []*repo.SourceRepo
}

type ctxKey struct{}

// New loads the source repositories declared in cfg and returns the App.
func New(cfg *conf.AyakaConfig) (*App, error) {
	app := &App{Config: cfg}
	for _, r := range cfg.Repos {
		repoconfig, err := conf.LoadSrcRepoConfig(r.Dir)
		if err != nil {
			return nil, errors.WrapErr(err, "failed to load source repository config "+r.Dir)
		}
		sr, err := repo.GetSrcRepo(r.Dir, SrcConfigFromConf(repoconfig))
		if err != nil {
			return nil, errors.WrapErr(err, "failed to load source repository "+r.Dir)
		}
		sr.Dir = r.Dir
		sr.DestDir = r.DestDir
		app.SrcRepos = append(app.SrcRepos, sr)
	}
	return app, nil
}

// WithContext stores app in ctx for From / FromContext to retrieve.
func WithContext(ctx context.Context, app *App) context.Context {
	return context.WithValue(ctx, ctxKey{}, app)
}

// From returns the App from the command context, or an empty App during
// shell completion (which runs before the App is built) to avoid a nil panic.
func From(cmd *cobra.Command) *App {
	return FromContext(cmd.Context())
}

// FromContext returns the App carried by ctx, or an empty App when none is set.
func FromContext(ctx context.Context) *App {
	if app, ok := ctx.Value(ctxKey{}).(*App); ok && app != nil {
		return app
	}
	return &App{}
}

// SrcConfigFromConf adapts conf.SrcRepoConfig to the conf-free repo.SrcConfig the domain layer uses.
func SrcConfigFromConf(c *conf.SrcRepoConfig) *repo.SrcConfig {
	if c == nil {
		return nil
	}
	sc := &repo.SrcConfig{
		Name:       c.Name,
		Maintainer: c.Maintainer,
		URL:        c.URL,
		Build:      c.Build,
	}
	// The repo maintainer is the natural PACKAGER when the build config leaves it
	// unset, matching how a hand-run makepkg picks up the packager's identity.
	if sc.Build.Makepkg.Packager == "" {
		sc.Build.Makepkg.Packager = c.Maintainer
	}
	sc.InstallPkgs.Files = c.InstallPkgs.Files
	sc.InstallPkgs.Names = c.InstallPkgs.Names
	return sc
}

func (a *App) GetSrcRepo(name string) *repo.SourceRepo {
	for _, r := range a.SrcRepos {
		if r.Config.Name == name {
			return r
		}
	}
	return nil
}

func (a *App) GetSrcRepoNames() []string {
	return lo.Map(a.SrcRepos, func(r *repo.SourceRepo, _ int) string {
		return r.Config.Name
	})
}
