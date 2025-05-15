package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

//go:embed out/**
var embeddedFiles embed.FS

func rootCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "lumine",
		Short: "Lumine is a frontend for Ayato",
		RunE: func(cmd *cobra.Command, args []string) error {
			staticFS, err := fs.Sub(embeddedFiles, "out")
			if err != nil {
				return fmt.Errorf("failed to prepare embedded filesystem: %w", err)
			}

			fileServer := http.FileServer(http.FS(staticFS))
			http.Handle("/", fileServer)

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

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
