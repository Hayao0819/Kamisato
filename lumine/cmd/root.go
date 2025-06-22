package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Hayao0819/Kamisato/lumine/embed"
	"github.com/spf13/cobra"
)

type LumineConfig struct {
	ServerUrl string `json:"server_url"`
}

func (l *LumineConfig) JSON() []byte {
	data, err := json.Marshal(l)
	if err != nil {
		return nil
	}
	return data
}

func RootCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "lumine",
		Short: "Lumine is a frontend for Ayato",
		RunE: func(cmd *cobra.Command, args []string) error {
			h, err := embed.NextHandler()
			if err != nil {
				return fmt.Errorf("failed to prepare embedded filesystem: %w", err)
			}

			config := LumineConfig{
				ServerUrl: "http://localhost:9090",
			}

			http.Handle("/", h)
			http.Handle("/env.json", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if _, err := w.Write(config.JSON()); err != nil {
					http.Error(w, "failed to write config", http.StatusInternalServerError)
				}
			}))

			cmd.PrintErrln("Starting Lumine server on", addr)
			if err := http.ListenAndServe(addr, nil); err != nil {
				return fmt.Errorf("failed to start server: %w", err)
			}
			return nil

		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVar(&addr, "addr", ":8080", "address to listen on")

	return cmd
}
