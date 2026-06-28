package hookcmd

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/internal/pacmanhook"
	"github.com/spf13/cobra"
)

func hookUninstallCmd() *cobra.Command {
	var dir, pacmanConf string
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the installed pacman hook",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dir == "" {
				dir = pacmanhook.HookDir(pacmanConf)
			}
			path, err := pacmanhook.Uninstall(dir, uploadHookFileName)
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
