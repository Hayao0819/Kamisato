package shared

import (
	"context"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

// App is the per-invocation dependency set for the ayaka command tree. It is
// built once in the root PersistentPreRunE and threaded to subcommands through
// the command context, so commands read their deps from here instead of mutable
// package globals; that keeps each command testable in isolation.
type App struct {
	Config   *conf.AyakaConfig
	SrcRepos []*repo.SourceRepo
}

type appCtxKey struct{}

// NewApp loads the source repositories declared in cfg and returns the App.
func NewApp(cfg *conf.AyakaConfig) (*App, error) {
	app := &App{Config: cfg}
	for _, r := range cfg.Repos {
		repoconfig, err := conf.LoadSrcRepoConfig(r.Dir)
		if err != nil {
			return nil, errwrap.WrapErr(err, "failed to load source repository config "+r.Dir)
		}
		sr, err := repo.GetSrcRepo(r.Dir, SrcConfigFromConf(repoconfig))
		if err != nil {
			return nil, errwrap.WrapErr(err, "failed to load source repository "+r.Dir)
		}
		sr.Dir = r.Dir
		sr.DestDir = r.DestDir
		app.SrcRepos = append(app.SrcRepos, sr)
	}
	return app, nil
}

// WithApp stores app in ctx for AppFrom / AppFromContext to retrieve.
func WithApp(ctx context.Context, app *App) context.Context {
	return context.WithValue(ctx, appCtxKey{}, app)
}

// AppFrom returns the App threaded through the command context. Shell completion
// runs before PersistentPreRunE, so no App is present then; callers get an empty
// App whose lookups return nothing rather than a nil-pointer panic.
func AppFrom(cmd *cobra.Command) *App {
	return AppFromContext(cmd.Context())
}

// AppFromContext returns the App carried by ctx, or an empty App when none is set.
func AppFromContext(ctx context.Context) *App {
	if app, ok := ctx.Value(appCtxKey{}).(*App); ok && app != nil {
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
		ArchBuild:  c.ArchBuild,
		Server:     c.Server,
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

func (a *App) GetDestDir(name string) string {
	if r := a.GetSrcRepo(name); r != nil {
		return r.DestDir
	}
	return ""
}

func (a *App) GetSrcDir(name string) string {
	if r := a.GetSrcRepo(name); r != nil {
		return r.Dir
	}
	return ""
}

func (a *App) GetSrcRepoNames() []string {
	return lo.Map(a.SrcRepos, func(r *repo.SourceRepo, _ int) string {
		return r.Config.Name
	})
}
