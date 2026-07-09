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

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/internal/gitcmd"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

const pinnedBranch = "kayo-pinned"

// Materialize (re)builds root/<pkgbase>.git as a bare repo whose HEAD is the
// reviewed commit, cloned from the already-checked-out sourceDir.
func Materialize(ctx context.Context, root, pkgbase, sourceDir, commit string) error {
	if commit == "" {
		return errors.NewErr("cannot materialize without a pinned commit")
	}
	if err := os.MkdirAll(root, 0o755); err != nil { //nolint:gosec // served git root is exposed over dumb-HTTP and is world-readable by design
		return errors.WrapErr(err, "failed to create served root")
	}
	repo := filepath.Join(root, pkgbase+".git")
	if err := os.RemoveAll(repo); err != nil {
		return errors.WrapErr(err, "failed to clear served repo")
	}

	if err := gitcmd.Clone(ctx, gitcmd.CloneOptions{URL: sourceDir, Dir: repo, Bare: true}); err != nil {
		return err
	}
	// Point HEAD at the reviewed commit so a clone checks out the pinned tree,
	// then refresh the dumb-HTTP index — all through go-git, no git process.
	if err := gitcmd.SetRef(repo, "refs/heads/"+pinnedBranch, commit); err != nil {
		return err
	}
	if err := gitcmd.SetHead(repo, "refs/heads/"+pinnedBranch); err != nil {
		return err
	}
	return gitcmd.UpdateServerInfo(repo)
}

func Remove(root, pkgbase string) error {
	return os.RemoveAll(filepath.Join(root, pkgbase+".git"))
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
