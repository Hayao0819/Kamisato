// Package gitserve implements sara's variant-B git serving: instead of
// redirecting a clone to the upstream (whose HEAD can move after review), sara
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

	"github.com/Hayao0819/Kamisato/internal/gitcmd"
	"github.com/Hayao0819/Kamisato/internal/utils"
)

const pinnedBranch = "sara-pinned"

// Materialize (re)builds root/<pkgbase>.git as a bare repo whose HEAD is the
// reviewed commit, cloned from the already-checked-out sourceDir. A git client
// cloning it receives exactly that commit.
func Materialize(ctx context.Context, root, pkgbase, sourceDir, commit string) error {
	if commit == "" {
		return utils.NewErr("cannot materialize without a pinned commit")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return utils.WrapErr(err, "failed to create served root")
	}
	repo := filepath.Join(root, pkgbase+".git")
	if err := os.RemoveAll(repo); err != nil {
		return utils.WrapErr(err, "failed to clear served repo")
	}

	if err := gitcmd.Clone(ctx, gitcmd.CloneOptions{URL: sourceDir, Dir: repo, Bare: true}); err != nil {
		return err
	}
	// Point HEAD at the reviewed commit so a clone checks out the pinned tree,
	// then refresh the dumb-HTTP index.
	if err := gitcmd.Run(ctx, repo, "update-ref", "refs/heads/"+pinnedBranch, commit); err != nil {
		return err
	}
	if err := gitcmd.Run(ctx, repo, "symbolic-ref", "HEAD", "refs/heads/"+pinnedBranch); err != nil {
		return err
	}
	return gitcmd.Run(ctx, repo, "update-server-info")
}

// Remove drops a served repo.
func Remove(root, pkgbase string) error {
	return os.RemoveAll(filepath.Join(root, pkgbase+".git"))
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
	if base := pkgbaseFromGitPath(r.URL.Path); base != "" {
		if st, err := os.Stat(filepath.Join(h.root, base+".git")); err == nil && st.IsDir() {
			h.files.ServeHTTP(w, r)
			return
		}
	}
	h.fallback.ServeHTTP(w, r)
}

// pkgbaseFromGitPath returns the pkgbase when the first path segment ends in
// ".git" (a clone request), else "".
func pkgbaseFromGitPath(p string) string {
	seg, _, _ := strings.Cut(strings.TrimPrefix(p, "/"), "/")
	if seg != ".git" && strings.HasSuffix(seg, ".git") {
		return strings.TrimSuffix(seg, ".git")
	}
	return ""
}
