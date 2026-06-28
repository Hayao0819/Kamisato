package cmd

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	utils "github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/lumine/embed"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

// lumineEnv is served at /env.json. AYATO_URL is the API base; "" = same origin.
// AUTH_MODE selects how the SPA authenticates: "cookie" (same-origin session
// cookie behind this BFF/proxy) or "bearer" (the SPA calls ayato cross-origin
// with a token; no proxy).
type lumineEnv struct {
	AyatoURL *string `json:"AYATO_URL"`
	AuthMode string  `json:"AUTH_MODE"`
	Fallback bool    `json:"FALLBACK"`
}

// newReverseProxy builds the ayato reverse proxy with a Rewrite hook instead of
// the default Director. The default Director appends to the inbound
// X-Forwarded-For, trusting spoofed values; instead we strip every
// client-supplied forwarding header, then SetXForwarded repopulates them from
// the real connection. SetURL must run before SetXForwarded so Host is correct.
// FlushInterval=-1 streams SSE through without buffering.
func newReverseProxy(target *url.URL) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		FlushInterval: -1,
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.Out.Header.Del("X-Forwarded-For")
			pr.Out.Header.Del("X-Forwarded-Host")
			pr.Out.Header.Del("X-Forwarded-Proto")
			pr.Out.Header.Del("Forwarded")
			pr.SetURL(target)
			pr.SetXForwarded()
		},
	}
}

func RootCmd() *cobra.Command {
	var addr string
	var ayatoURL string
	var authMode string
	var debug bool
	cmd := &cobra.Command{
		Use:   "lumine",
		Short: "Lumine is a frontend for Ayato",
		RunE: func(cmd *cobra.Command, args []string) error {
			if debug {
				utils.UseColorLog(slog.LevelDebug)
				gin.SetMode(gin.DebugMode)
			} else {
				utils.UseColorLog(slog.LevelInfo)
				gin.SetMode(gin.ReleaseMode)
			}

			static, err := embed.NextHandler()
			if err != nil {
				return utils.WrapErr(err, "failed to prepare embedded filesystem")
			}

			if ayatoURL == "" {
				ayatoURL = os.Getenv("LUMINE_AYATO_URL")
			}

			engine := gin.New()
			engine.Use(gin.Recovery())
			engine.Use(utils.GinLog())

			var env lumineEnv
			switch authMode {
			case "bearer":
				// Bearer mode: no proxy. The static SPA calls ayato cross-origin
				// and authenticates with a token, so ayato stays a distinct origin
				// the SPA addresses directly.
				if ayatoURL == "" {
					return fmt.Errorf("--auth-mode bearer requires --ayato-url (the cross-origin ayato URL the SPA calls directly)")
				}
				env = lumineEnv{AyatoURL: &ayatoURL, AuthMode: "bearer"}
			case "cookie", "":
				// Cookie mode: front ayato same-origin so the session cookie is
				// first-party. With no --ayato-url the SPA has no API base.
				if ayatoURL != "" {
					target, err := url.Parse(ayatoURL)
					if err != nil {
						return fmt.Errorf("invalid ayato url %q: %w", ayatoURL, err)
					}
					proxy := newReverseProxy(target)
					forward := func(c *gin.Context) {
						proxy.ServeHTTP(c.Writer, c.Request)
					}
					engine.Any("/api/*proxyPath", forward)
					engine.Any("/repo/*proxyPath", forward)
					same := ""
					env = lumineEnv{AyatoURL: &same, AuthMode: "cookie"}
				} else {
					env = lumineEnv{AuthMode: "cookie"}
				}
			default:
				return fmt.Errorf("invalid --auth-mode %q (want \"cookie\" or \"bearer\")", authMode)
			}

			envJSON, err := json.Marshal(env)
			if err != nil {
				return utils.WrapErr(err, "failed to encode env")
			}
			engine.Any("/env.json", func(c *gin.Context) {
				if c.GetHeader("Sec-Fetch-Site") != "same-origin" {
					c.String(http.StatusForbidden, "forbidden")
					return
				}
				c.Data(http.StatusOK, "application/json", envJSON)
			})

			engine.NoRoute(gin.WrapH(static))

			slog.Info("Waiting on address", "addr", addr)
			if err := engine.Run(addr); err != nil {
				return err
			}
			return nil
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVar(&addr, "addr", ":8080", "address to listen on")
	cmd.Flags().StringVar(&ayatoURL, "ayato-url", "", "Ayato URL to proxy /api and /repo to (env: LUMINE_AYATO_URL)")
	cmd.Flags().StringVar(&authMode, "auth-mode", "cookie", "auth delivery mode: cookie (same-origin BFF proxy) or bearer (SPA calls ayato cross-origin with a token)")
	cmd.Flags().BoolVarP(&debug, "debug", "d", false, "Enable debug mode")

	return cmd
}
