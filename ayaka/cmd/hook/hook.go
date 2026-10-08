package hookcmd

import (
	installcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/hook/install"
	uninstallcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/hook/uninstall"
	uploadcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/hook/upload"
	"github.com/spf13/cobra"
)

// Cmd manages the pacman PostTransaction hook for build-once-share-many: a
// locally-built package lands in the ayato repo so other machines pull the binary.
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Manage the pacman hook that uploads installed packages to ayato",
	}
	cmd.AddCommand(installcmd.Cmd(), uninstallcmd.Cmd(), uploadcmd.Cmd())
	return cmd
}
