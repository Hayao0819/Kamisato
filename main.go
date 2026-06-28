package main

import (
	"os"

	ayaka "github.com/Hayao0819/Kamisato/ayaka/cmd"
	ayato "github.com/Hayao0819/Kamisato/ayato/cmd"
	kayo "github.com/Hayao0819/Kamisato/kayo/cmd"
	lumine "github.com/Hayao0819/Kamisato/lumine/cmd"
	miko "github.com/Hayao0819/Kamisato/miko/cmd"
	"github.com/spf13/cobra"
)

func rootCmd() *cobra.Command {
	cmd := cobra.Command{
		Use: "kamisato",
	}

	cmd.AddCommand(ayaka.RootCmd())
	cmd.AddCommand(ayato.RootCmd())
	cmd.AddCommand(lumine.RootCmd())
	cmd.AddCommand(miko.RootCmd())
	cmd.AddCommand(kayo.RootCmd())
	return &cmd
}

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
