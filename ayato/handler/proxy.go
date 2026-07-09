package handler

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/internal/auth/apikey"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

// Reverse-proxies build/job requests to the internal miko server; clients never
// reach miko directly and ayato authenticates with the shared API key.
type MikoProxy struct {
	proxy *httputil.ReverseProxy
}

// Returns nil when no upstream is configured so the router skips the miko routes.
func (h *Handler) MikoProxy() (*MikoProxy, error) {
	return NewMikoProxy(h.cfg)
}

// Returns nil when no upstream is configured so the router can skip the routes.
func NewMikoProxy(cfg *conf.AyatoConfig) (*MikoProxy, error) {
	if cfg == nil || cfg.Miko.URL == "" {
		return nil, nil
	}

	target, err := url.Parse(cfg.Miko.URL)
	if err != nil {
		return nil, err
	}

	apiKey := cfg.Miko.APIKey
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
			// Drop any client-supplied key before setting ours so a client
			// cannot smuggle its own X-API-Key through to miko.
			req.Header.Del(apikey.Header)
			// Strip the user's Authorization; leaving it would leak their CLI
			// token to miko, which authenticates only via the shared X-API-Key.
			req.Header.Del("Authorization")
			// The end-user's session cookie must never cross into miko (which
			// runs with docker.sock); miko authenticates only via X-API-Key.
			req.Header.Del("Cookie")
			if apiKey != "" {
				req.Header.Set(apikey.Header, apiKey)
			}
		},
		// Flush every write immediately so SSE job logs stream to the client
		// instead of being buffered until the response completes.
		FlushInterval: -1,
	}

	return &MikoProxy{proxy: proxy}, nil
}

// For routes with no dynamic segment; proxies to a fixed upstream path.
func (p *MikoProxy) Handler(upstreamPath string) gin.HandlerFunc {
	return p.HandlerFunc(func(*gin.Context) string { return upstreamPath })
}

// Pinning the upstream path (not copying the client path) stops a client from
// steering the proxy at an arbitrary miko endpoint.
func (p *MikoProxy) HandlerFunc(build func(c *gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.URL.Path = build(c)
		p.proxy.ServeHTTP(c.Writer, c.Request)
	}
}
