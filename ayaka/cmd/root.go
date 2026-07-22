package cmd

import (
	"log/slog"

	"github.com/Hayao0819/nahi/cobrautils"
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	admincmd "github.com/Hayao0819/Kamisato/ayaka/cmd/admin"
	aurcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/aur"
	buildcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/build"
	bumpcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/bump"
	hookcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/hook"
	initcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/init"
	keycmd "github.com/Hayao0819/Kamisato/ayaka/cmd/key"
	keyringcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/keyring"
	listcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/list"
	mikocmd "github.com/Hayao0819/Kamisato/ayaka/cmd/miko"
	plancmd "github.com/Hayao0819/Kamisato/ayaka/cmd/plan"
	prunecmd "github.com/Hayao0819/Kamisato/ayaka/cmd/prune"
	repocmd "github.com/Hayao0819/Kamisato/ayaka/cmd/repo"
	servercmd "github.com/Hayao0819/Kamisato/ayaka/cmd/server"
	srcinfocmd "github.com/Hayao0819/Kamisato/ayaka/cmd/srcinfo"
	statuscmd "github.com/Hayao0819/Kamisato/ayaka/cmd/status"
	submodulescmd "github.com/Hayao0819/Kamisato/ayaka/cmd/submodules"
	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/version"
)

func RootCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:           "ayaka",
		Short:         "Repository management tool",
		Long:          "Ayaka is a tool for managing pacman repositories.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			configFile, _ := cmd.Flags().GetString("config")
			c, err := conf.LoadAyakaConfigFrom(configFile, cmd.Flags())
			if err != nil {
				return err
			}

			level := slog.LevelInfo
			if c.Debug {
				level = slog.LevelDebug
			}
			cliutil.Setup(level, cliutil.ColorEnabled(cmd))

			a, err := app.New(c)
			if err != nil {
				return err
			}
			cmd.SetContext(app.WithContext(cmd.Context(), a))
			return nil
		},
	}

	cliutil.SetVersion(&cmd)
	cliutil.AddNoColorFlag(&cmd)

	subCmds := cobrautils.Registory{}
	subCmds.Add(
		repocmd.Cmd(),
		aurcmd.Cmd(),
		buildcmd.Cmd(),
		plancmd.Cmd(),
		bumpcmd.Cmd(),
		prunecmd.Cmd(),
		mikocmd.Cmd(),
		hookcmd.Cmd(),
		admincmd.Cmd(),
		statuscmd.Cmd(),
		srcinfocmd.Cmd(),
		listcmd.Cmd(),
		initcmd.Cmd(),
		submodulescmd.Cmd(),
		servercmd.Cmd(),
		keycmd.Cmd(),
		keyringcmd.Cmd(),
		version.Command(),
	)
	subCmds.Bind(&cmd)

	cmd.PersistentFlags().StringP("config", "c", "", "Explicit config file path")
	cmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug mode")

	return &cmd
}
