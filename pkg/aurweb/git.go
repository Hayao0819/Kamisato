package aurweb

import (
	"net/http"
	"strings"
)

// GitClone redirects git clone requests to the Backend's SourceURL (managed) or Upstream's git base (unmanaged);
// the host never proxies pack data.
func (s *Server) GitClone(w http.ResponseWriter, r *http.Request) {
	base := PkgbaseFromGitPath(r.URL.Path)
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
	// #nosec G710 -- target origin comes from admin-validated source data or operator federation config.
	http.Redirect(w, r, loc, http.StatusFound)
}

// Snapshot redirects cgit snapshot tarballs for unmanaged packages to the upstream;
// managed packages reject snapshots (no built tree on this host).
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
	// #nosec G710 -- the operator configures the origin and SnapshotURL path-escapes base.
	http.Redirect(w, r, s.upstream.SnapshotURL(base), http.StatusFound)
}

// PlainPKGBUILD redirects cgit raw-PKGBUILD previews to the upstream (the redirect-only host keeps no built tree).
func (s *Server) PlainPKGBUILD(w http.ResponseWriter, r *http.Request) {
	h := r.URL.Query().Get("h")
	if h == "" || s.upstream == nil {
		http.NotFound(w, r)
		return
	}
	// #nosec G710 -- the operator configures the origin and PlainURL query-escapes h.
	http.Redirect(w, r, s.upstream.PlainURL(h), http.StatusFound)
}

// PkgbaseFromGitPath returns the pkgbase when the first path segment ends in
// ".git" (a clone request), or "" otherwise.
func PkgbaseFromGitPath(p string) string {
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
