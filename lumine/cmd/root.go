package cmd

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/Hayao0819/Kamisato/internal/conf"
	utils "github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/internal/weblog"
	"github.com/Hayao0819/Kamisato/lumine/embed"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

// lumineEnv is served at /env.json. AYATO_URL "" means same origin. AUTH_MODE is
// "cookie" (same-origin session cookie via this proxy) or "bearer" (SPA calls
// ayato cross-origin with a token, no proxy).
type lumineEnv struct {
	AyatoURL *string `json:"AYATO_URL"`
	AuthMode string  `json:"AUTH_MODE"`
	Fallback bool    `json:"FALLBACK"`
}

// newReverseProxy uses a Rewrite hook, not the default Director: the Director
// appends to inbound X-Forwarded-For (trusting spoofed values), so we strip the
// client-supplied forwarding headers and let SetXForwarded refill them from the
// real connection. SetURL must precede SetXForwarded so Host is correct, and
// FlushInterval=-1 streams SSE without buffering.
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
	cmd := &cobra.Command{
		Use:   "lumine",
		Short: "Lumine is a frontend for Ayato",
		RunE: func(cmd *cobra.Command, args []string) error {
			configFile, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			cfg, err := conf.LoadLumineConfig(cmd.Flags(), configFile)
			if err != nil {
				return err
			}

			if cfg.Debug {
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

			engine := gin.New()
			engine.Use(gin.Recovery())
			engine.Use(weblog.GinLog())

			// Validate has already rejected any auth_mode other than cookie/bearer
			// and ensured bearer has an ayato_url.
			var env lumineEnv
			switch cfg.AuthMode {
			case "bearer":
				// Bearer mode: no proxy; the SPA calls ayato cross-origin with a token.
				env = lumineEnv{AyatoURL: &cfg.AyatoURL, AuthMode: "bearer"}
			default:
				// Cookie mode: proxy ayato same-origin so the session cookie stays first-party.
				if cfg.AyatoURL != "" {
					target, err := url.Parse(cfg.AyatoURL)
					if err != nil {
						return fmt.Errorf("invalid ayato url %q: %w", cfg.AyatoURL, err)
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

			slog.Info("Waiting on address", "addr", cfg.Addr)
			if err := engine.Run(cfg.Addr); err != nil {
				return err
			}
			return nil
		},
		SilenceUsage: true,
	}

	cmd.Flags().String("addr", ":8080", "address to listen on")
	cmd.Flags().String("ayato-url", "", "Ayato URL to proxy /api and /repo to (env: LUMINE_AYATO_URL)")
	cmd.Flags().String("auth-mode", "cookie", "auth delivery mode: cookie (same-origin BFF proxy) or bearer (SPA calls ayato cross-origin with a token)")
	cmd.Flags().BoolP("debug", "d", false, "Enable debug mode")
	cmd.Flags().StringP("config", "c", "", "Config file")

	return cmd
}
