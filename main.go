package main

import (
	"os"

	"github.com/spf13/cobra"

	ayaka "github.com/Hayao0819/Kamisato/ayaka/cmd"
	ayato "github.com/Hayao0819/Kamisato/ayato/cmd"
	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	kayo "github.com/Hayao0819/Kamisato/kayo/cmd"
	lumine "github.com/Hayao0819/Kamisato/lumine/cmd"
	miko "github.com/Hayao0819/Kamisato/miko/cmd"
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
	cmd.AddCommand(cmdline.VersionCommand())
	cmdline.SetVersion(&cmd)
	return &cmd
}

func main() {
	os.Exit(cmdline.Execute(rootCmd()))
}
