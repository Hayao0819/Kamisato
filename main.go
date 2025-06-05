package main

import (
	"os"

	ayaka "github.com/Hayao0819/Kamisato/ayaka/cmd"
	ayato "github.com/Hayao0819/Kamisato/ayato/cmd"
	lumine "github.com/Hayao0819/Kamisato/lumine/cmd"
	"github.com/spf13/cobra"
)

func rootCmd() *cobra.Command {
	cmd := cobra.Command{
		Use: "kamisato",
	}

	cmd.AddCommand(ayaka.RootCmd())
	cmd.AddCommand(ayato.RootCmd())
	cmd.AddCommand(lumine.RootCmd())
	return &cmd
}

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
