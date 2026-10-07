package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/internal/api/client"
	"github.com/Hayao0819/Kamisato/internal/errors"
	httpserver "github.com/Hayao0819/Kamisato/internal/http/server"
	lumineconfig "github.com/Hayao0819/Kamisato/lumine/config"
	"github.com/Hayao0819/Kamisato/lumine/embed"
)

type lumineEnv struct {
	AyatoURL    *string `json:"AYATO_URL"`
	AuthMode    string  `json:"AUTH_MODE"`
	Fallback    bool    `json:"FALLBACK"`
	Title       string  `json:"TITLE,omitempty"`
	Description string  `json:"DESCRIPTION,omitempty"`
}

func bearerLumineEnv(target *url.URL) lumineEnv {
	normalized := target.String()
	return lumineEnv{AyatoURL: &normalized, AuthMode: "bearer"}
}

func newReverseProxy(target *url.URL) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		FlushInterval: -1,
		Rewrite: func(proxyRequest *httputil.ProxyRequest) {
			proxyRequest.Out.Header.Del("X-Forwarded-For")
			proxyRequest.Out.Header.Del("X-Forwarded-Host")
			proxyRequest.Out.Header.Del("X-Forwarded-Proto")
			proxyRequest.Out.Header.Del("Forwarded")
			proxyRequest.SetURL(target)
			proxyRequest.SetXForwarded()
		},
	}
}

func Run(ctx context.Context, cfg *lumineconfig.LumineConfig) error {
	static, err := embed.NextHandler()
	if err != nil {
		return errors.WrapErr(err, "failed to prepare embedded filesystem")
	}
	engine := httpserver.NewEngine()

	var target *url.URL
	if cfg.AyatoURL != "" {
		target, err = client.ParseBaseURL(cfg.AyatoURL)
		if err != nil {
			return errors.WrapErr(err, "invalid ayato url "+cfg.AyatoURL)
		}
	}
	var env lumineEnv
	if cfg.AuthMode == "bearer" {
		env = bearerLumineEnv(target)
	} else if target != nil {
		proxy := newReverseProxy(target)
		forward := func(c *gin.Context) {
			proxy.ServeHTTP(c.Writer, c.Request)
		}
		engine.Any("/api/*proxyPath", forward)
		engine.Any("/repo/*proxyPath", forward)
		sameOrigin := ""
		env = lumineEnv{AyatoURL: &sameOrigin, AuthMode: "cookie"}
	} else {
		env = lumineEnv{AuthMode: "cookie"}
	}
	env.Title = cfg.Title
	env.Description = cfg.Description

	envJSON, err := json.Marshal(env)
	if err != nil {
		return errors.WrapErr(err, "failed to encode env")
	}
	engine.Any("/env.json", func(c *gin.Context) {
		if c.GetHeader("Sec-Fetch-Site") != "same-origin" {
			c.String(http.StatusForbidden, "forbidden")
			return
		}
		c.Data(http.StatusOK, "application/json", envJSON)
	})
	engine.NoRoute(gin.WrapH(static))

	slog.Info("Waiting on address", "addr", cfg.Addr)
	return httpserver.ServeHTTP(ctx, httpserver.NewServer(cfg.Addr, engine), nil)
}
