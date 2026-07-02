package hook

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/pkg/pacman/alpm"
	"github.com/spf13/cobra"
)

// UninstallCmd builds the shared "hook uninstall" command for a tool's hook
// file. kayo and ayaka install different hooks but remove them identically.
func UninstallCmd(fileName string) *cobra.Command {
	var dir, pacmanConf string
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the installed pacman hook",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dir == "" {
				dir = alpm.HookDir(pacmanConf)
			}
			path, err := Uninstall(dir, fileName)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", path)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "hook directory (default: pacman.conf HookDir)")
	cmd.Flags().StringVar(&pacmanConf, "pacman-config", "", "pacman.conf path for resolving HookDir")
	return cmd
}
