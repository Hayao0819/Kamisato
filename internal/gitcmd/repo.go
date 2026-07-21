package gitcmd

import (
	"context"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/go-git/go-billy/v6/osfs"
	git "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/cache"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/storage/filesystem"
)

// HeadCommit returns the HEAD commit hash of the repo in dir, via go-git (no git
// process spawned). It replaces `git rev-parse HEAD`.
func HeadCommit(_ context.Context, dir string) (string, error) {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return "", errors.WrapErr(err, "open repo "+dir)
	}
	head, err := repo.Head()
	if err != nil {
		return "", errors.WrapErr(err, "resolve HEAD")
	}
	return head.Hash().String(), nil
}

// RepoRoot returns the working-tree root of the repo containing dir, the go-git
// equivalent of `git rev-parse --show-toplevel`.
func RepoRoot(dir string) (string, error) {
	repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return "", errors.WrapErr(err, "open repo "+dir)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return "", errors.WrapErr(err, "open worktree")
	}
	return wt.Filesystem().Root(), nil
}

// ChangedFiles returns the paths that differ between the from and to revisions,
// the go-git equivalent of `git diff --name-only from to`.
func ChangedFiles(dir, from, to string) ([]string, error) {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return nil, errors.WrapErr(err, "open repo "+dir)
	}
	fromTree, err := revTree(repo, from)
	if err != nil {
		return nil, err
	}
	toTree, err := revTree(repo, to)
	if err != nil {
		return nil, err
	}
	changes, err := fromTree.Diff(toTree)
	if err != nil {
		return nil, errors.WrapErr(err, "diff trees")
	}
	seen := map[string]struct{}{}
	var names []string
	for _, c := range changes {
		for _, n := range []string{c.From.Name, c.To.Name} {
			if n == "" {
				continue
			}
			if _, ok := seen[n]; ok {
				continue
			}
			seen[n] = struct{}{}
			names = append(names, n)
		}
	}
	return names, nil
}

func revTree(repo *git.Repository, rev string) (*object.Tree, error) {
	hash, err := repo.ResolveRevision(plumbing.Revision(rev))
	if err != nil {
		return nil, errors.WrapErr(err, "resolve "+rev)
	}
	commit, err := repo.CommitObject(*hash)
	if err != nil {
		return nil, errors.WrapErr(err, "load commit "+rev)
	}
	return commit.Tree()
}

// CommitPaths stages the worktree-relative paths and commits them, the go-git
// equivalent of `git add -- <paths> && git commit -m <message>`. The author
// comes from git config; an unset user.name/email fails the commit.
func CommitPaths(dir string, paths []string, message string) (string, error) {
	repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return "", errors.WrapErr(err, "open repo "+dir)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return "", errors.WrapErr(err, "open worktree")
	}
	// Commit snapshots the whole index, so refuse when unrelated changes are
	// already staged rather than sweeping them into this commit.
	st, err := wt.Status()
	if err != nil {
		return "", errors.WrapErr(err, "read worktree status")
	}
	allowed := map[string]struct{}{}
	for _, p := range paths {
		allowed[p] = struct{}{}
	}
	for file, fs := range st {
		if _, ok := allowed[file]; ok {
			continue
		}
		if fs.Staging != git.Unmodified && fs.Staging != git.Untracked {
			return "", errors.NewErr("index already has staged changes: " + file)
		}
	}
	for _, p := range paths {
		if _, err := wt.Add(p); err != nil {
			return "", errors.WrapErr(err, "stage "+p)
		}
	}
	hash, err := wt.Commit(message, &git.CommitOptions{})
	if err != nil {
		return "", errors.WrapErr(err, "commit")
	}
	return hash.String(), nil
}

// SetRef points refName at hash in the repo at dir, the go-git equivalent of
// `git update-ref`.
func SetRef(dir, refName, hash string) error {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return errors.WrapErr(err, "open repo "+dir)
	}
	return repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.NewHash(hash)))
}

// SetHead points HEAD at the symbolic target ref in the repo at dir, the go-git
// equivalent of `git symbolic-ref HEAD <target>`.
func SetHead(dir, target string) error {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return errors.WrapErr(err, "open repo "+dir)
	}
	return repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.ReferenceName(target)))
}

// UpdateServerInfo refreshes the dumb-HTTP info files (info/refs,
// objects/info/packs) of the bare repo at dir, the go-git equivalent of
// `git update-server-info`.
func UpdateServerInfo(dir string) error {
	fs := osfs.New(dir)
	storer := filesystem.NewStorage(fs, cache.NewObjectLRUDefault())
	return transport.UpdateServerInfo(storer, fs)
}
