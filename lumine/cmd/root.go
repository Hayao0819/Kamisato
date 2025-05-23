package cmd

import (
	"fmt"
	"net/http"

	"github.com/Hayao0819/Kamisato/lumine/embed"
	"github.com/spf13/cobra"
)

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

			http.Handle("/", h)
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
