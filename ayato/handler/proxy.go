package handler

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/internal/auth/apikey"
	"github.com/Hayao0819/Kamisato/internal/client"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

// Reverse-proxies build/job requests to the internal miko server; clients never
// reach miko directly and ayato authenticates with the shared API key.
type MikoProxy struct {
	proxy  *httputil.ReverseProxy
	target *url.URL
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

	target, err := client.ParseBaseURL(cfg.Miko.URL)
	if err != nil {
		return nil, err
	}

	apiKey := cfg.Miko.APIKey
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
			req.Header.Del(apikey.Header)
			req.Header.Del("Authorization")
			req.Header.Del("Cookie")
			req.Header.Del("X-Log-Token")
			if apiKey != "" {
				req.Header.Set(apikey.Header, apiKey)
			}
		},
		FlushInterval: -1,
	}

	return &MikoProxy{proxy: proxy, target: target}, nil
}

// Handler proxies to a fixed upstream path expressed as data segments.
func (p *MikoProxy) Handler(segments ...string) gin.HandlerFunc {
	return p.HandlerFunc(func(*gin.Context) []string { return segments })
}

// Pinning the upstream path (not copying the client path) stops a client from
// steering the proxy at an arbitrary miko endpoint.
func (p *MikoProxy) HandlerFunc(build func(c *gin.Context) []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		target := client.EndpointURL(p.target, build(c)...)
		c.Request.URL.Path = target.Path
		c.Request.URL.RawPath = target.RawPath
		c.Request.URL.RawQuery = ""
		p.proxy.ServeHTTP(c.Writer, c.Request)
	}
}
