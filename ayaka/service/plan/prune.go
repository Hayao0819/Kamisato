package plan

import (
	"context"
	"log/slog"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/pacman/repo"
	"github.com/Hayao0819/Kamisato/internal/pacman/source"
)

type PruneOptions struct {
	Arch        string
	DatabaseURL string
	DryRun      bool
	Remove      func(context.Context, string, string) error
}

func Prune(ctx context.Context, src *source.SourceRepo, options PruneOptions) ([]string, error) {
	return prune(ctx, src, options, repo.RepoFromURL)
}

func prune(
	ctx context.Context,
	src *source.SourceRepo,
	options PruneOptions,
	fetch func(string, string) (*repo.RemoteRepo, error),
) ([]string, error) {
	if src == nil || src.Config == nil {
		return nil, errors.NewErr("source repo is not configured")
	}

	var desired []string
	for _, sourcePackage := range src.Pkgs {
		desired = append(desired, sourcePackage.Names()...)
	}
	if len(desired) == 0 {
		return nil, errors.NewErr("source repo " + src.Config.Name + " has no packages; refusing to prune")
	}

	databaseURL := options.DatabaseURL
	if databaseURL == "" && src.Config.URL != "" {
		databaseURL = strings.TrimRight(src.Config.URL, "/") + "/" + options.Arch
	}
	if databaseURL == "" {
		return nil, errors.NewErr("source repo " + src.Config.Name + " has no url in repo.json; pass --diff-url")
	}

	remote, err := fetch(databaseURL, src.Config.Name)
	if errors.Is(err, repo.ErrRepoNotFound) {
		slog.Info("remote repo db not found; nothing to prune", "url", databaseURL)
		return nil, nil
	}
	if err != nil {
		return nil, errors.WrapErr(err, "failed to read remote repo db")
	}

	packages := repo.PrunablePackages(desired, remote)
	if len(packages) == 0 {
		slog.Info("nothing to prune", "repo", src.Config.Name, "arch", options.Arch)
		return nil, nil
	}
	if options.DryRun {
		slog.Info("prunable packages removed from source", "repo", src.Config.Name, "arch", options.Arch, "count", len(packages))
		return packages, nil
	}
	if options.Remove == nil {
		return nil, errors.NewErr("package remover is not configured")
	}

	slog.Info("pruning packages removed from source", "repo", src.Config.Name, "arch", options.Arch, "packages", packages)
	for _, name := range packages {
		if err := options.Remove(ctx, src.Config.Name, name); err != nil {
			return nil, err
		}
	}
	return packages, nil
}
