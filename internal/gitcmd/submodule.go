package gitcmd

import (
	"context"
	"path/filepath"

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

// UpdateSubmodules updates the submodules of the repo at dir via go-git — the
// equivalent of `git submodule update` with the given flags. remote advances each
// checked-out submodule to the tip of its configured branch (or origin's default
// when unset) instead of the pinned commit.
func UpdateSubmodules(ctx context.Context, dir string, init, recursive, remote bool) error {
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
	if !remote {
		return nil
	}
	for _, s := range subs {
		c := s.Config()
		subdir := filepath.Join(dir, c.Path)
		if _, err := git.PlainOpen(subdir); err != nil {
			continue // not checked out yet; skip
		}
		if c.Branch != "" {
			if err := SyncHard(ctx, subdir, c.Branch); err != nil {
				return errors.WrapErr(err, "update submodule "+c.Name+" to remote")
			}
			continue
		}
		if err := Pull(ctx, subdir); err != nil {
			return errors.WrapErr(err, "update submodule "+c.Name+" to remote")
		}
	}
	return nil
}
