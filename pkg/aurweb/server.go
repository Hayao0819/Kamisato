package aurweb

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/Hayao0819/Kamisato/pkg/ratelimit"
)

// Server turns a Backend (and optional Upstream) into the aurweb HTTP surface; safe for concurrent use.
type Server struct {
	backend   Backend
	upstream  Upstream
	log       *slog.Logger
	dumps     dumpCache
	limiter   ratelimit.Limiter          // nil unless a limiter option is set
	policy    ratelimit.Policy           // fixed-window budget for RPC requests
	limiterFn func(*http.Request) string // client key for the limiter (defaults to remoteIP)
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

// WithLimiter injects a rate limiter and fixed-window policy. A nil key function
// keys requests on the remote IP.
func WithLimiter(
	limiter ratelimit.Limiter,
	policy ratelimit.Policy,
	keyFn func(*http.Request) string,
) Option {
	return func(s *Server) {
		if limiter != nil && policy.Enabled() {
			if keyFn == nil {
				keyFn = remoteIP
			}
			s.limiter = limiter
			s.policy = policy
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
