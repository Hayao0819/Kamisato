package buildset

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Hayao0819/Kamisato/internal/filesystem/safefile"
	"github.com/Hayao0819/Kamisato/internal/pacman/builder"
	"github.com/Hayao0819/Kamisato/internal/pacman/host"
	"github.com/Hayao0819/Kamisato/internal/pacman/repo"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

type preparedPlan struct {
	plan                *Plan
	workspace           string
	pacmanConf          string
	localRepositoryDirs []string
	sources             map[string]*sourceRecord
}

func New(dependencies Dependencies) *Application {
	if dependencies.AUR == nil {
		dependencies.AUR = aurweb.NewAURUpstream("")
	}
	if dependencies.Clone == nil {
		dependencies.Clone = defaultCloneSource
	}
	if dependencies.LoadRepositories == nil {
		dependencies.LoadRepositories = loadRepositories
	}
	if dependencies.LoadPacmanConfig == nil {
		dependencies.LoadPacmanConfig = host.LoadBuildConfig
	}
	if dependencies.RepoTool == nil {
		dependencies.RepoTool = repo.NativeTool{}
	}
	if dependencies.Metadata == nil {
		if metadata, ok := dependencies.Backend.(builder.MetadataBackend); ok {
			dependencies.Metadata = metadata
		}
	}
	return &Application{
		aur:              dependencies.AUR,
		backend:          dependencies.Backend,
		metadata:         dependencies.Metadata,
		clone:            dependencies.Clone,
		loadPacmanConfig: dependencies.LoadPacmanConfig,
		loadRepositories: dependencies.LoadRepositories,
		repoTool:         dependencies.RepoTool,
	}
}

func (a *Application) Plan(ctx context.Context, request PlanRequest) (*Plan, error) {
	prepared, cleanup, err := a.prepare(ctx, request)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return prepared.plan, nil
}

func (a *Application) prepare(ctx context.Context, request PlanRequest) (*preparedPlan, func(), error) {
	if err := validateTargetArchitecture(request.Arch); err != nil {
		return nil, func() {}, err
	}
	requested, err := normalizePackages(request.Packages)
	if err != nil {
		return nil, func() {}, err
	}
	if len(request.Packages) == 0 && len(request.LocalSources) == 0 {
		return &preparedPlan{plan: &Plan{
			SchemaVersion: SchemaVersion,
			Arch:          request.Arch,
			Install:       []string{},
			BuildOrder:    []string{},
			Builds:        []PlannedBuild{},
		}}, func() {}, nil
	}
	if request.WorkDir != "" {
		if err := os.MkdirAll(request.WorkDir, 0o700); err != nil {
			return nil, func() {}, fmt.Errorf("create work directory %q: %w", request.WorkDir, err)
		}
	}
	workspace, err := os.MkdirTemp(request.WorkDir, "ayaka-build-*")
	if err != nil {
		return nil, func() {}, fmt.Errorf("create build workspace: %w", err)
	}
	cleanup := func() {
		if request.KeepWork {
			slog.Info("keeping build workspace", "path", workspace)
			return
		}
		_ = os.RemoveAll(workspace)
	}
	fail := func(err error) (*preparedPlan, func(), error) {
		cleanup()
		return nil, func() {}, err
	}

	config, err := a.loadPacmanConfig(request.PacmanConf, request.Arch)
	if err != nil {
		return fail(err)
	}
	pacmanConf := filepath.Join(workspace, "pacman.conf")
	if err := safefile.WriteFile(pacmanConf, config.PacmanConf, 0o644); err != nil {
		return fail(fmt.Errorf("stage pacman config: %w", err))
	}
	repositories, err := a.loadRepositories(ctx, config)
	if err != nil {
		return fail(err)
	}
	localRepositoryDirs, err := configuredLocalRepositoryDirs(config)
	if err != nil {
		return fail(err)
	}
	sourcesDir := filepath.Join(workspace, "sources")
	local, err := a.collectLocalSources(ctx, request.LocalSources, sourcesDir, request.Arch, pacmanConf, localRepositoryDirs)
	if err != nil {
		return fail(err)
	}
	planner := newPlanner(ctx, a, request.Arch, sourcesDir, pacmanConf, localRepositoryDirs, repositories, local)
	plan, err := planner.build(requested)
	if err != nil {
		return fail(err)
	}
	if len(plan.Builds) == 0 {
		cleanup()
		return &preparedPlan{plan: plan}, func() {}, nil
	}
	return &preparedPlan{
		plan: plan, workspace: workspace, pacmanConf: pacmanConf,
		localRepositoryDirs: localRepositoryDirs, sources: planner.byBase,
	}, cleanup, nil
}
