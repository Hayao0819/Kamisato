package gitcmd

import (
	"context"

	"github.com/Hayao0819/Kamisato/internal/errors"

	git "github.com/go-git/go-git/v6"
)

// AddSubmodule adds url as a submodule at path (relative to the repo at dir) via
// go-git — the equivalent of `git submodule add <url> <path>`.
func AddSubmodule(ctx context.Context, dir, url, path string) error {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return errors.WrapErr(err, "open repo "+dir)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return errors.WrapErr(err, "open worktree")
	}
	if _, err := wt.AddSubmoduleContext(ctx, &git.SubmoduleAddOptions{URL: url, Path: path}); err != nil {
		return errors.WrapErr(err, "git submodule add "+url)
	}
	return nil
}

// UpdateSubmodules checks out the submodules of the repo at dir via go-git at
// their recorded commits — the equivalent of `git submodule update`. Advancing
// a checkout to its origin is PullPackages' job.
func UpdateSubmodules(ctx context.Context, dir string, init, recursive bool) error {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return errors.WrapErr(err, "open repo "+dir)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return errors.WrapErr(err, "open worktree")
	}
	subs, err := wt.Submodules()
	if err != nil {
		return errors.WrapErr(err, "list submodules")
	}
	recurse := git.NoRecurseSubmodules
	if recursive {
		recurse = git.DefaultSubmoduleRecursionDepth
	}
	if err := subs.UpdateContext(ctx, &git.SubmoduleUpdateOptions{Init: init, RecurseSubmodules: recurse}); err != nil {
		return errors.WrapErr(err, "git submodule update")
	}
	return nil
}
