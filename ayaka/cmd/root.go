package cmd

import (
	"log/slog"

	admincmd "github.com/Hayao0819/Kamisato/ayaka/cmd/admin"
	aurcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/aur"
	buildcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/build"
	hookcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/hook"
	initcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/init"
	listcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/list"
	mikocmd "github.com/Hayao0819/Kamisato/ayaka/cmd/miko"
	repocmd "github.com/Hayao0819/Kamisato/ayaka/cmd/repo"
	servercmd "github.com/Hayao0819/Kamisato/ayaka/cmd/server"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	srcinfocmd "github.com/Hayao0819/Kamisato/ayaka/cmd/srcinfo"
	statuscmd "github.com/Hayao0819/Kamisato/ayaka/cmd/status"
	submodulescmd "github.com/Hayao0819/Kamisato/ayaka/cmd/submodules"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/nahi/cobrautils"
	"github.com/spf13/cobra"
)

func RootCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "ayaka",
		Short: "Repository management tool",
		Long:  "Ayaka is a tool for managing pacman repositories.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			c, err := conf.LoadAyakaConfig(cmd.Flags())
			if err != nil {
				return err
			}
			shared.Config = c

			if shared.Config.Debug {
				utils.UseColorLog(slog.LevelDebug)
			} else {
				utils.UseColorLog(slog.LevelInfo)
			}

			if err := shared.InitSrcRepos(); err != nil {
				return err
			}

			return nil
		},
		SilenceUsage: true,
		CompletionOptions: cobra.CompletionOptions{
			HiddenDefaultCmd: true,
		},
		SilenceErrors: true,
	}

	subCmds := cobrautils.Registory{}
	subCmds.Add(
		repocmd.Cmd(),
		aurcmd.Cmd(),
		buildcmd.Cmd(),
		mikocmd.Cmd(),
		hookcmd.Cmd(),
		admincmd.Cmd(),
		statuscmd.Cmd(),
		srcinfocmd.Cmd(),
		listcmd.Cmd(),
		initcmd.Cmd(),
		submodulescmd.Cmd(),
		servercmd.Cmd(),
	)
	subCmds.Bind(&cmd)

	cmd.PersistentFlags().StringP("repodir", "", "", "Repository directory")
	cmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug mode")

	return &cmd
}
