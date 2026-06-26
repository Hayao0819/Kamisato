package aurweb

import (
	"net/http"
	"strings"
)

// GitClone redirects "git clone <host>/<pkgbase>.git" to where the source lives:
// the Backend's SourceURL for managed packages, the Upstream's git base
// otherwise. The host never proxies pack data.
func (s *Server) GitClone(w http.ResponseWriter, r *http.Request) {
	base := pkgbaseFromGitPath(r.URL.Path)
	if base == "" {
		http.NotFound(w, r)
		return
	}

	target, ok, err := s.backend.SourceURL(r.Context(), base)
	if err != nil {
		s.log.Error("aurweb: backend SourceURL failed", "pkgbase", base, "error", err)
	}
	if !ok {
		if s.upstream == nil {
			http.NotFound(w, r)
			return
		}
		target = s.upstream.GitBase() + "/" + base + ".git"
	}

	loc := strings.TrimRight(target, "/")
	if rest := gitPathRemainder(r.URL.Path); rest != "" {
		loc += "/" + rest
	}
	if r.URL.RawQuery != "" {
		loc += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, loc, http.StatusFound)
}

// Snapshot redirects the cgit snapshot tarball for unmanaged packages to the
// upstream. Managed packages are served via git clone, not tarball, since the
// host keeps no built tree.
func (s *Server) Snapshot(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/cgit/aur.git/snapshot/")
	base := strings.TrimSuffix(name, ".tar.gz")
	if base == "" {
		http.NotFound(w, r)
		return
	}

	if _, ok, _ := s.backend.SourceURL(r.Context(), base); ok {
		http.Error(w, "snapshot unavailable; clone "+base+".git instead", http.StatusNotFound)
		return
	}
	if s.upstream == nil {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, s.upstream.SnapshotURL(base), http.StatusFound)
}

// PlainPKGBUILD redirects the cgit raw-PKGBUILD preview to the upstream. Helpers
// use it only for the `-G`/print preview; managed previews are out of scope for
// the redirect-only MVP.
func (s *Server) PlainPKGBUILD(w http.ResponseWriter, r *http.Request) {
	h := r.URL.Query().Get("h")
	if h == "" || s.upstream == nil {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, s.upstream.PlainURL(h), http.StatusFound)
}

// pkgbaseFromGitPath returns the pkgbase when the first path segment ends in
// ".git" (a smart-HTTP clone request), or "" otherwise.
func pkgbaseFromGitPath(p string) string {
	seg, _, _ := strings.Cut(strings.TrimPrefix(p, "/"), "/")
	if seg != ".git" && strings.HasSuffix(seg, ".git") {
		return strings.TrimSuffix(seg, ".git")
	}
	return ""
}

// gitPathRemainder returns the path after the "<pkgbase>.git" segment, e.g.
// "info/refs" for "/<pkgbase>.git/info/refs".
func gitPathRemainder(p string) string {
	_, rest, _ := strings.Cut(strings.TrimPrefix(p, "/"), "/")
	return strings.Trim(rest, "/")
}
