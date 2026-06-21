package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/Hayao0819/Kamisato/lumine/embed"
	"github.com/spf13/cobra"
)

// lumineEnv is served at /env.json and consumed by the web app's APIClient.
// AYATO_URL is the base the browser uses: "" means same origin (lumine proxies
// /api and /repo to ayato); null lets the user set it from the UI when
// SERVER_CONFIGURABLE is true.
type lumineEnv struct {
	AyatoURL           *string `json:"AYATO_URL"`
	ServerConfigurable bool    `json:"SERVER_CONFIGURABLE"`
	Fallback           bool    `json:"FALLBACK"`
}

func RootCmd() *cobra.Command {
	var addr string
	var ayatoURL string
	cmd := &cobra.Command{
		Use:   "lumine",
		Short: "Lumine is a frontend for Ayato",
		RunE: func(cmd *cobra.Command, args []string) error {
			h, err := embed.NextHandler()
			if err != nil {
				return fmt.Errorf("failed to prepare embedded filesystem: %w", err)
			}

			if ayatoURL == "" {
				ayatoURL = os.Getenv("LUMINE_AYATO_URL")
			}

			mux := http.NewServeMux()

			var env lumineEnv
			if ayatoURL != "" {
				// Proxy /api and /repo to ayato so the browser only ever talks to
				// lumine (same origin): no CORS, no cross-origin reachability issues.
				target, err := url.Parse(ayatoURL)
				if err != nil {
					return fmt.Errorf("invalid ayato url %q: %w", ayatoURL, err)
				}
				proxy := httputil.NewSingleHostReverseProxy(target)
				proxy.FlushInterval = -1 // stream SSE job logs as they arrive
				mux.Handle("/api/", proxy)
				mux.Handle("/repo/", proxy)
				same := ""
				env = lumineEnv{AyatoURL: &same}
			} else {
				// No upstream: the user sets the ayato URL from the UI.
				env = lumineEnv{ServerConfigurable: true}
			}

			envJSON, err := json.Marshal(env)
			if err != nil {
				return fmt.Errorf("failed to encode env: %w", err)
			}
			mux.HandleFunc("/env.json", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if _, err := w.Write(envJSON); err != nil {
					http.Error(w, "failed to write config", http.StatusInternalServerError)
				}
			})
			mux.Handle("/", h)

			cmd.PrintErrln("Starting Lumine server on", addr)
			if err := http.ListenAndServe(addr, mux); err != nil {
				return fmt.Errorf("failed to start server: %w", err)
			}
			return nil
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVar(&addr, "addr", ":8080", "address to listen on")
	cmd.Flags().StringVar(&ayatoURL, "ayato-url", "", "Ayato URL to proxy /api and /repo to (env: LUMINE_AYATO_URL); unset = configure it from the UI")

	return cmd
}
