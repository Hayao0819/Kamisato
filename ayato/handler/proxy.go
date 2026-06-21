package handler

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/Hayao0819/Kamisato/internal/apikey"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/gin-gonic/gin"
)

// MikoProxy reverse-proxies build/job requests to the internal miko build
// server. Clients never reach miko directly; ayato authenticates to it with the
// shared API key. miko owns job state, so ayato is a pure pass-through here.
type MikoProxy struct {
	proxy *httputil.ReverseProxy
}

// MikoProxy builds the reverse proxy to miko from the handler's config. It
// returns nil when no upstream is configured so the router skips the routes.
func (h *Handler) MikoProxy() (*MikoProxy, error) {
	return NewMikoProxy(h.cfg)
}

// NewMikoProxy builds a reverse proxy targeting cfg.Miko.URL. It returns nil
// when no upstream is configured so the router can skip wiring the routes.
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

// Handler proxies the current request to miko under a fixed upstream path.
// Use it for routes with no dynamic segment.
func (p *MikoProxy) Handler(upstreamPath string) gin.HandlerFunc {
	return p.HandlerFunc(func(*gin.Context) string { return upstreamPath })
}

// HandlerFunc proxies the current request to miko, computing the upstream path
// per request via build. Pinning the path here (rather than copying the client
// path) keeps a client from steering the proxy at an arbitrary miko endpoint;
// the request body, query, method, and headers pass through.
func (p *MikoProxy) HandlerFunc(build func(c *gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.URL.Path = build(c)
		p.proxy.ServeHTTP(c.Writer, c.Request)
	}
}
