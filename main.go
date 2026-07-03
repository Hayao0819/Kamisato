package main

import (
	"os"

	ayaka "github.com/Hayao0819/Kamisato/ayaka/cmd"
	ayato "github.com/Hayao0819/Kamisato/ayato/cmd"
	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/version"
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
	cmd.AddCommand(version.Command())
	cliutil.SetVersion(&cmd)
	return &cmd
}

func main() {
	os.Exit(cliutil.Execute(rootCmd()))
}
