package aurweb

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// Server turns a Backend (and optional Upstream) into the aurweb HTTP surface.
// It is safe for concurrent use. Mount it directly (it implements http.Handler)
// or wire its individual handlers (RPC, GitClone, ...) into another router.
type Server struct {
	backend  Backend
	upstream Upstream
	log      *slog.Logger
	dumps    dumpCache
	limiter  *rateLimiter // nil unless WithRateLimit is set
}

type Option func(*Server)

// WithUpstream sets the fallback for packages the Backend does not manage. A
// Server without an Upstream is a closed, private instance.
func WithUpstream(u Upstream) Option { return func(s *Server) { s.upstream = u } }

// WithLogger sets the structured logger (defaults to slog.Default()).
func WithLogger(l *slog.Logger) Option {
	return func(s *Server) {
		if l != nil {
			s.log = l
		}
	}
}

func New(backend Backend, opts ...Option) *Server {
	s := &Server{backend: backend, log: slog.Default()}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ServeHTTP routes the entire aurweb surface.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	switch {
	case p == "/rpc" || p == "/rpc/" || p == "/rpc.php" || strings.HasPrefix(p, "/rpc/"):
		s.RPC(w, r)
	case strings.HasPrefix(p, "/cgit/aur.git/snapshot/"):
		s.Snapshot(w, r)
	case strings.HasPrefix(p, "/cgit/aur.git/plain/"):
		s.PlainPKGBUILD(w, r)
	case p == "/packages-meta-ext-v1.json.gz":
		s.serveMetaDump(w, r, true)
	case p == "/packages-meta-v1.json.gz":
		s.serveMetaDump(w, r, false)
	case p == "/packages.gz":
		s.serveNamesDump(w, r)
	case pkgbaseFromGitPath(p) != "":
		s.GitClone(w, r)
	default:
		http.NotFound(w, r)
	}
}

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
	if s.limiter != nil && !s.limiter.allow(s.limiter.keyFn(r)) {
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

	arg := firstArg(q.args)
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
	arg := firstArg(q.args)

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
	if len(names) > 20 {
		names = names[:20]
	}
	if names == nil {
		names = []string{}
	}
	s.writeJSON(w, r, q.callback, names)
}

func (s *Server) writeResults(w http.ResponseWriter, r *http.Request, callback, typ string, pkgs []Pkg, info bool) {
	results := make([]map[string]any, len(pkgs))
	for i, p := range pkgs {
		results[i] = p.toMap(info)
	}
	s.writeJSON(w, r, callback, map[string]any{
		"version":     Version,
		"type":        typ,
		"resultcount": len(results),
		"results":     results,
	})
}

func (s *Server) writeError(w http.ResponseWriter, r *http.Request, callback string, version int, msg string) {
	s.writeJSON(w, r, callback, map[string]any{
		"version":     versionOrNull(version),
		"type":        "error",
		"resultcount": 0,
		"results":     []any{},
		"error":       msg,
	})
}

// versionOrNull renders the RPC version field: the client's value, or JSON null
// when it was omitted (version 0), the way aurweb echoes it.
func versionOrNull(version int) any {
	if version == 0 {
		return nil
	}
	return version
}

func (s *Server) writeJSON(w http.ResponseWriter, r *http.Request, callback string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		s.log.Error("aurweb: marshal response", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	sum := md5.Sum(body)
	etag := `"` + hex.EncodeToString(sum[:]) + `"`
	w.Header().Set("ETag", etag)
	if match := r.Header.Get("If-None-Match"); match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// aurweb answers with HTTP 200 even for app-level errors. Set it explicitly:
	// a host may have preset another status (gin's NoRoute defaults to 404).
	if callback != "" {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("/**/" + callback + "("))
		_, _ = w.Write(body)
		_, _ = w.Write([]byte(")"))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// rpcQuery is a parsed RPC request from any of the supported request shapes.
type rpcQuery struct {
	version  int
	typ      string
	by       By
	args     []string
	callback string
}

func parseRPC(r *http.Request) rpcQuery {
	_ = r.ParseForm()
	var q rpcQuery

	path := r.URL.Path
	var pathArg string
	if strings.HasPrefix(path, "/rpc/v") {
		segs := strings.Split(strings.Trim(strings.TrimPrefix(path, "/rpc/"), "/"), "/")
		if len(segs) > 0 {
			q.version = parseVersion(segs[0])
		}
		if len(segs) > 1 {
			q.typ = segs[1]
		}
		if len(segs) > 2 {
			pathArg = strings.Join(segs[2:], "/")
		}
		if r.Method == http.MethodPost && strings.Contains(r.Header.Get("Content-Type"), "json") {
			if data, _ := readAllLimited(r.Body); len(data) > 0 {
				var body struct {
					By  string          `json:"by"`
					Arg json.RawMessage `json:"arg"`
				}
				if json.Unmarshal(data, &body) == nil {
					if body.By != "" {
						q.by = By(body.By)
					}
					q.args = append(q.args, parseJSONArg(body.Arg)...)
				}
			}
		}
	} else {
		q.version = atoiSafe(r.Form.Get("v"))
		q.typ = r.Form.Get("type")
	}

	q.callback = r.Form.Get("callback")
	if by := r.Form.Get("by"); by != "" {
		q.by = By(by)
	}
	if vals, ok := r.Form["arg[]"]; ok {
		q.args = append(q.args, vals...)
	}
	if a := r.Form.Get("arg"); a != "" {
		q.args = append(q.args, a)
	}
	// aurweb ignores the OpenAPI path arg when query/body args are present.
	if len(q.args) == 0 && pathArg != "" {
		q.args = append(q.args, pathArg)
	}
	return q
}

// parseJSONArg accepts both the string and []string spellings of the OpenAPI
// POST "arg" field.
func parseJSONArg(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var list []string
	if json.Unmarshal(raw, &list) == nil {
		return list
	}
	var single string
	if json.Unmarshal(raw, &single) == nil && single != "" {
		return []string{single}
	}
	return nil
}

// resolveType maps aurweb's type aliases to a canonical handler key.
func resolveType(t string) string {
	switch t {
	case "info", "multiinfo":
		return "multiinfo"
	case "search":
		return "search"
	case "msearch":
		return "msearch"
	case "suggest":
		return "suggest"
	case "suggest-pkgbase":
		return "suggest-pkgbase"
	default:
		return ""
	}
}

// mergeUnique concatenates lists, keeping the first occurrence of each distinct
// key. Earlier lists win, so callers pass the higher-precedence list first.
func mergeUnique[T any, K comparable](key func(T) K, lists ...[]T) []T {
	seen := map[K]bool{}
	var out []T
	for _, list := range lists {
		for _, v := range list {
			k := key(v)
			if seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, v)
		}
	}
	return out
}

func mergeByName(local, upstream []Pkg) []Pkg {
	return mergeUnique(func(p Pkg) string { return p.Name }, local, upstream)
}

func mergeStrings(local, upstream []string) []string {
	return mergeUnique(func(s string) string { return s }, local, upstream)
}

func dedupeNonEmpty(in []string) []string {
	seen := make(map[string]bool, len(in))
	var out []string
	for _, v := range in {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func firstArg(args []string) string {
	for _, a := range args {
		if a != "" {
			return a
		}
	}
	return ""
}

func parseVersion(seg string) int {
	return atoiSafe(strings.TrimPrefix(seg, "v"))
}

func atoiSafe(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}
