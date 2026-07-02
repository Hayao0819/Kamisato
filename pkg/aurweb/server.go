package aurweb

import (
	"log/slog"
	"net/http"
	"strings"
)

// Server turns a Backend (and optional Upstream) into the aurweb HTTP surface.
// It is safe for concurrent use. Mount it directly (it implements http.Handler)
// or wire its individual handlers (RPC, GitClone, ...) into another router.
type Server struct {
	backend   Backend
	upstream  Upstream
	log       *slog.Logger
	dumps     dumpCache
	limiter   Limiter                    // nil unless a limiter option is set
	limiterFn func(*http.Request) string // client key for the limiter (defaults to remoteIP)
}

// Limiter is the /rpc + NoRoute per-client throttle. It is an interface so a
// caller can inject a shared (kv-backed, cross-replica) limiter; the built-in
// per-instance one from WithRateLimit satisfies it too.
type Limiter interface {
	// Allow records one request for client and reports whether it is within limit.
	Allow(client string) bool
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

// WithLimiter injects a rate limiter (e.g. a shared kv-backed one that holds the
// limit across replicas) and the key function that identifies the client. A nil
// keyFn keys on the request's remote IP.
func WithLimiter(l Limiter, keyFn func(*http.Request) string) Option {
	return func(s *Server) {
		if l != nil {
			if keyFn == nil {
				keyFn = remoteIP
			}
			s.limiter = l
			s.limiterFn = keyFn
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
	case PkgbaseFromGitPath(p) != "":
		s.GitClone(w, r)
	default:
		http.NotFound(w, r)
	}
}
