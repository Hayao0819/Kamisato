package hookcmd

import (
	"github.com/spf13/cobra"

	installcmd "github.com/Hayao0819/Kamisato/kayo/cmd/hook/install"
	uninstallcmd "github.com/Hayao0819/Kamisato/kayo/cmd/hook/uninstall"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Manage the pacman PreTransaction hook that runs 'kayo verify'",
	}
	cmd.AddCommand(installcmd.Cmd(), uninstallcmd.Cmd())
	return cmd
}
