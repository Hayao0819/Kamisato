package aurweb

import (
	"cmp"
	"net/http"
	"regexp"
	"strings"
)

var callbackRe = regexp.MustCompile(`^[a-zA-Z0-9()_.]{1,128}$`)

// maxResults caps info/search results, matching aurweb's max_rpc_results.
const maxResults = 5000

// RPC handles the aurweb /rpc endpoint (legacy GET/POST and the OpenAPI-style
// /rpc/v5/* routes).
func (s *Server) RPC(w http.ResponseWriter, r *http.Request) {
	// A bare GET /rpc (no query) serves the human-readable docs, like aurweb. The
	// OpenAPI /rpc/v5/* routes carry their args in the path, so exclude them.
	if r.Method == http.MethodGet && r.URL.RawQuery == "" && !strings.HasPrefix(r.URL.Path, "/rpc/v") {
		s.serveRPCDoc(w, r)
		return
	}

	q := parseRPC(r)

	// aurweb checks the rate limit before anything else, so an over-limit request
	// always gets 429 even with a bad callback.
	if s.limiter != nil && !s.limiter.Allow(s.limiterFn(r)) {
		s.writeRateLimited(w, q.version)
		return
	}
	if q.callback != "" && !callbackRe.MatchString(q.callback) {
		s.writeError(w, r, "", q.version, "Invalid callback name.")
		return
	}
	if q.version == 0 {
		s.writeError(w, r, q.callback, q.version, "Please specify an API version.")
		return
	}
	if q.version != Version {
		s.writeError(w, r, q.callback, q.version, "Invalid version specified.")
		return
	}

	switch resolveType(q.typ) {
	case "multiinfo":
		s.handleInfo(w, r, q)
	case "search":
		s.handleSearch(w, r, q, false)
	case "msearch":
		s.handleSearch(w, r, q, true)
	case "suggest":
		s.handleSuggest(w, r, q, false)
	case "suggest-pkgbase":
		s.handleSuggest(w, r, q, true)
	default:
		s.writeError(w, r, q.callback, q.version, "Incorrect request type specified.")
	}
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request, q rpcQuery) {
	ctx := r.Context()
	names := dedupeNonEmpty(q.args)
	if len(names) == 0 {
		s.writeError(w, r, q.callback, q.version, "No request type/data specified.")
		return
	}

	local, err := s.backend.Info(ctx, names)
	if err != nil {
		s.log.Error("aurweb: backend info failed", "error", err)
	}

	results := local
	if s.upstream != nil {
		found := make(map[string]bool, len(local))
		for _, p := range local {
			found[p.Name] = true
		}
		var missing []string
		for _, n := range names {
			if !found[n] {
				missing = append(missing, n)
			}
		}
		if len(missing) > 0 {
			up, uerr := s.upstream.Info(ctx, missing)
			if uerr != nil {
				s.log.Warn("aurweb: upstream info failed", "error", uerr)
			} else {
				results = append(results, up...)
			}
		}
	}
	if len(results) > maxResults {
		s.writeError(w, r, q.callback, q.version, "Too many package results.")
		return
	}
	s.writeResults(w, r, q.callback, "multiinfo", results, true)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request, q rpcQuery, maintainer bool) {
	ctx := r.Context()
	by := q.by
	if maintainer {
		by = ByMaintainer
	}
	if by == "" {
		by = DefaultBy
	}
	if !by.valid() {
		s.writeError(w, r, q.callback, q.version, "Incorrect by field specified.")
		return
	}

	arg := cmp.Or(q.args...)
	if by != ByMaintainer {
		if arg == "" {
			s.writeError(w, r, q.callback, q.version, "No request type/data specified.")
			return
		}
		if len([]rune(arg)) < 2 {
			s.writeError(w, r, q.callback, q.version, "Query arg too small.")
			return
		}
	}

	local, err := s.backend.Search(ctx, by, arg)
	if err != nil {
		s.log.Error("aurweb: backend search failed", "error", err)
	}

	results := local
	if s.upstream != nil {
		up, uerr := s.upstream.Search(ctx, by, arg)
		if uerr != nil {
			s.log.Warn("aurweb: upstream search failed", "error", uerr)
		} else {
			results = mergeByName(local, up)
		}
	}
	if len(results) > maxResults {
		s.writeError(w, r, q.callback, q.version, "Too many package results.")
		return
	}
	s.writeResults(w, r, q.callback, "search", results, false)
}

func (s *Server) handleSuggest(w http.ResponseWriter, r *http.Request, q rpcQuery, pkgbase bool) {
	ctx := r.Context()
	arg := cmp.Or(q.args...)

	local, err := s.backend.Suggest(ctx, arg, pkgbase)
	if err != nil {
		s.log.Error("aurweb: backend suggest failed", "error", err)
	}

	names := local
	if s.upstream != nil {
		up, uerr := s.upstream.Suggest(ctx, arg, pkgbase)
		if uerr != nil {
			s.log.Warn("aurweb: upstream suggest failed", "error", uerr)
		} else {
			names = mergeStrings(local, up)
		}
	}
	if len(names) > SuggestLimit {
		names = names[:SuggestLimit]
	}
	if names == nil {
		names = []string{}
	}
	s.writeJSON(w, r, q.callback, names)
}
