package cmd

import (
	"log/slog"

	buildcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/build"
	cicmd "github.com/Hayao0819/Kamisato/ayaka/cmd/ci"
	hookcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/hook"
	initcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/init"
	keycmd "github.com/Hayao0819/Kamisato/ayaka/cmd/key"
	keyringcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/keyring"
	mikocmd "github.com/Hayao0819/Kamisato/ayaka/cmd/miko"
	plancmd "github.com/Hayao0819/Kamisato/ayaka/cmd/plan"
	repocmd "github.com/Hayao0819/Kamisato/ayaka/cmd/repo"
	prunecmd "github.com/Hayao0819/Kamisato/ayaka/cmd/repo/prune"
	servercmd "github.com/Hayao0819/Kamisato/ayaka/cmd/server"
	srccmd "github.com/Hayao0819/Kamisato/ayaka/cmd/src"
	bumpcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/src/bump"
	srcinfocmd "github.com/Hayao0819/Kamisato/ayaka/cmd/src/srcinfo"
	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	"github.com/Hayao0819/nahi/cobrautils"
	"github.com/spf13/cobra"
)

// grouped assigns cmds to the root help section id and returns them.
func grouped(id string, cmds ...*cobra.Command) []*cobra.Command {
	for _, c := range cmds {
		c.GroupID = id
	}
	return cmds
}

// deprecatedStub hides cmd at its old top-level path and points at the new one;
// it still runs, so existing scripts keep working for one transition release.
func deprecatedStub(cmd *cobra.Command, newPath string) *cobra.Command {
	cmd.Hidden = true
	cmd.Deprecated = "use '" + newPath + "'"
	return cmd
}

func RootCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:           "ayaka",
		Short:         "Repository management tool",
		Long:          "Ayaka is a tool for managing pacman repositories.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		level := slog.LevelInfo
		debug, err := cmd.PersistentFlags().GetBool("debug")
		if err != nil {
			return err
		}
		if debug {
			level = slog.LevelDebug
		}
		cmdline.Setup(level, cmdline.ColorEnabled(&cmd))
		return nil
	}

	cmdline.SetVersion(&cmd)
	cmdline.AddNoColorFlag(&cmd)

	cmd.AddGroup(
		&cobra.Group{ID: "src", Title: "Source repository:"},
		&cobra.Group{ID: "build", Title: "Build:"},
		&cobra.Group{ID: "ayato", Title: "Ayato server:"},
		&cobra.Group{ID: "signing", Title: "Signing:"},
		&cobra.Group{ID: "ci", Title: "CI:"},
	)

	subCmds := cobrautils.Registory{}
	subCmds.Add(grouped("src", initcmd.Cmd(), srccmd.Cmd())...)
	subCmds.Add(grouped("build", buildcmd.Cmd(), plancmd.Cmd(), mikocmd.Cmd())...)
	subCmds.Add(grouped("ayato", repocmd.Cmd(), servercmd.Cmd(), hookcmd.Cmd())...)
	subCmds.Add(grouped("signing", keycmd.Cmd(), keyringcmd.Cmd())...)
	subCmds.Add(grouped("ci", cicmd.Cmd())...)
	subCmds.Add(
		deprecatedStub(bumpcmd.Cmd(), "ayaka src bump"),
		deprecatedStub(srcinfocmd.Cmd(), "ayaka src srcinfo"),
		deprecatedStub(prunecmd.Cmd(), "ayaka repo prune"),
		cmdline.VersionCommand(),
	)
	subCmds.Bind(&cmd)

	cmd.PersistentFlags().StringP("config", "c", "", "Explicit config file path")
	cmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug mode")

	return &cmd
}
