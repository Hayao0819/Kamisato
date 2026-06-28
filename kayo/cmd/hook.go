package cmd

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/pacman/hook"
	"github.com/spf13/cobra"
)

//go:embed kayo-verify.hook.tmpl
var hookTemplate string

const hookFileName = "kayo-verify.hook"

func hookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Manage the pacman PreTransaction hook that runs 'kayo verify'",
	}
	cmd.AddCommand(hookInstallCmd(), hookUninstallCmd())
	return cmd
}

func hookInstallCmd() *cobra.Command {
	var dir, configPath, pacmanConf string
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the pacman hook (writes to a system dir; usually needs root)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			self, err := os.Executable()
			if err != nil {
				return utils.WrapErr(err, "cannot resolve the kayo binary path")
			}
			if err := hook.ValidateExecArg("kayo binary path", self); err != nil {
				return err
			}
			if err := hook.ValidateExecArg("--config-path", configPath); err != nil {
				return err
			}
			exec := self + " verify"
			if configPath != "" {
				exec += " -c " + configPath
			}

			if dir == "" {
				dir = hook.Dir(pacmanConf)
			}
			path, err := hook.Install(dir, hookFileName, hookTemplate, exec)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "installed %s\n", path)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "hook directory (default: pacman.conf HookDir)")
	cmd.Flags().StringVar(&configPath, "config-path", "", "kayo config path to bake into the hook's Exec")
	cmd.Flags().StringVar(&pacmanConf, "pacman-config", "", "pacman.conf path for resolving HookDir")
	return cmd
}

func hookUninstallCmd() *cobra.Command {
	var dir, pacmanConf string
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the installed pacman hook",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dir == "" {
				dir = hook.Dir(pacmanConf)
			}
			path, err := hook.Uninstall(dir, hookFileName)
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
