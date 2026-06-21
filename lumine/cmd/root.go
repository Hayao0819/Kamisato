package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/Hayao0819/Kamisato/lumine/embed"
	"github.com/spf13/cobra"
)

// lumineEnv is served at /env.json and consumed by the web app's APIClient.
// AYATO_URL is the ayato base URL the browser talks to; null lets the user set
// it from the UI (stored in localStorage) when SERVER_CONFIGURABLE is true.
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
			env := lumineEnv{ServerConfigurable: true}
			if ayatoURL != "" {
				env.AyatoURL = &ayatoURL
			}
			envJSON, err := json.Marshal(env)
			if err != nil {
				return fmt.Errorf("failed to encode env: %w", err)
			}

			http.Handle("/", h)
			http.HandleFunc("/env.json", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if _, err := w.Write(envJSON); err != nil {
					http.Error(w, "failed to write config", http.StatusInternalServerError)
				}
			})

			cmd.PrintErrln("Starting Lumine server on", addr)
			if err := http.ListenAndServe(addr, nil); err != nil {
				return fmt.Errorf("failed to start server: %w", err)
			}
			return nil
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVar(&addr, "addr", ":8080", "address to listen on")
	cmd.Flags().StringVar(&ayatoURL, "ayato-url", "", "Ayato base URL advertised to the browser (env: LUMINE_AYATO_URL)")

	return cmd
}
