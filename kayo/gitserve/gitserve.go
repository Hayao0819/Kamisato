// Package gitserve implements kayo's variant-B git serving: instead of
// redirecting a clone to the upstream (whose HEAD can move after review), kayo
// serves an approved package's reviewed commit from its own cache. The served
// tree is exactly what was audited and pinned, so "what was audited" equals
// "what gets built". Repos are served read-only over dumb HTTP (plain static
// files), which a git client clones without any server-side CGI.
package gitserve

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/internal/vcs/git"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

const pinnedBranch = "kayo-pinned"

// Materialize (re)builds root/<pkgbase>.git as a bare repo whose HEAD is the
// reviewed commit, cloned from the already-checked-out sourceDir.
func Materialize(ctx context.Context, root, pkgbase, sourceDir, commit string) error {
	repo, err := repoPath(root, pkgbase)
	if err != nil {
		return err
	}
	if commit == "" {
		return errors.NewErr("cannot materialize without a pinned commit")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o755); err != nil { //nolint:gosec // served git root is exposed over dumb-HTTP and is world-readable by design
		return errors.WrapErr(err, "failed to create served root")
	}
	staging, err := os.MkdirTemp(root, ".kayo-pin-*")
	if err != nil {
		return errors.WrapErr(err, "failed to create pin staging directory")
	}
	defer func() {
		if staging != "" {
			_ = os.RemoveAll(staging)
		}
	}()

	// Prepare the full replacement before touching the existing pin. A failed
	// clone, unreachable commit, or cancelled review must not discard it.
	next := filepath.Join(staging, "repo")
	// A reviewed checkout may have detached HEAD and only remote-tracking refs.
	// Mirror keeps their objects available even after the temporary source closes.
	if err := git.Clone(ctx, git.CloneOptions{URL: sourceDir, Dir: next, Mirror: true}); err != nil {
		return err
	}
	// Point HEAD at the reviewed commit so a clone checks out the pinned tree,
	// then refresh the dumb-HTTP index — all through go-git, no git process.
	if err := git.SetRef(next, "refs/heads/"+pinnedBranch, commit); err != nil {
		return err
	}
	if err := git.SetHead(next, "refs/heads/"+pinnedBranch); err != nil {
		return err
	}
	if err := git.UpdateServerInfo(next); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	previous := filepath.Join(staging, "previous")
	hadPrevious := false
	if err := os.Rename(repo, previous); err == nil {
		hadPrevious = true
	} else if !os.IsNotExist(err) {
		return errors.WrapErr(err, "failed to retain previous pin")
	}
	if err := os.Rename(next, repo); err != nil {
		if hadPrevious {
			if rollbackErr := os.Rename(previous, repo); rollbackErr != nil {
				// Keep the backup available for recovery if restoring also fails.
				staging = ""
				return errors.Join(errors.WrapErr(err, "failed to publish pin"), errors.WrapErr(rollbackErr, "previous pin retained at "+previous))
			}
		}
		return errors.WrapErr(err, "failed to publish pin")
	}
	return nil
}

func Remove(root, pkgbase string) error {
	repo, err := repoPath(root, pkgbase)
	if err != nil {
		return err
	}
	return os.RemoveAll(repo)
}

// Package bases come from both command arguments and repository metadata. Check
// confinement before any filesystem effects, especially recursive removal.
func repoPath(root, pkgbase string) (string, error) {
	if root == "" {
		return "", errors.NewErr("served git root is required")
	}
	if pkgbase == "" || !filepath.IsLocal(pkgbase) || filepath.Base(pkgbase) != pkgbase || strings.ContainsAny(pkgbase, "\\\x00") || pkgbase == "." {
		return "", errors.NewErrf("invalid package base %q", pkgbase)
	}
	return filepath.Join(root, pkgbase+".git"), nil
}

// MaterializePins (re)serves every pkgbase in sources at its approved commit,
// reconciling the served root with the trust store's pins. pin returns the
// approved commit, or ok=false to leave a pkgbase unserved so it falls through to
// the upstream redirect. Best-effort: an unreachable approved commit is logged via
// the joined error and skipped, not fatal. Returns the number materialized.
func MaterializePins(ctx context.Context, root string, sources map[string]string, pin func(pkgbase string) (string, bool)) (int, error) {
	var served int
	var errs []error
	for pkgbase, dir := range sources {
		commit, ok := pin(pkgbase)
		if !ok || commit == "" {
			continue
		}
		if err := Materialize(ctx, root, pkgbase, dir, commit); err != nil {
			errs = append(errs, errors.WrapErr(err, "pin "+pkgbase))
			continue
		}
		served++
	}
	return served, errors.Join(errs...)
}

// Handler serves materialized repos as static files and delegates everything
// else (RPC, cgit, dumps, unmanaged git redirects) to fallback.
type Handler struct {
	root     string
	files    http.Handler
	fallback http.Handler
}

func NewHandler(root string, fallback http.Handler) *Handler {
	return &Handler{root: root, files: http.FileServer(http.Dir(root)), fallback: fallback}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if base := aurweb.PkgbaseFromGitPath(r.URL.Path); base != "" {
		//nolint:gosec // base is one path segment from PkgbaseFromGitPath (no separators); FileServer(http.Dir) also confines traversal
		if st, err := os.Stat(filepath.Join(h.root, base+".git")); err == nil && st.IsDir() {
			h.files.ServeHTTP(w, r)
			return
		}
	}
	h.fallback.ServeHTTP(w, r)
}
