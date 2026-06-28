package cmd

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/spf13/cobra"
)

//go:embed kayo-verify.hook.tmpl
var hookTemplate string

const (
	defaultHookDir = "/usr/share/libalpm/hooks"
	hookFileName   = "kayo-verify.hook"
)

func hookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Manage the pacman PreTransaction hook that runs 'kayo verify'",
	}
	cmd.AddCommand(hookInstallCmd(), hookUninstallCmd())
	return cmd
}

func hookInstallCmd() *cobra.Command {
	var dir, configPath string
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the pacman hook (writes to a system dir; usually needs root)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			self, err := os.Executable()
			if err != nil {
				return utils.WrapErr(err, "cannot resolve the kayo binary path")
			}
			exec := self + " verify"
			if configPath != "" {
				exec = self + " verify -c " + configPath
			}

			content := strings.ReplaceAll(hookTemplate, "@EXEC@", exec)
			path := filepath.Join(dir, hookFileName)
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return utils.WrapErr(err, "failed to write hook (root needed for "+dir+"?)")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "installed %s\n", path)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", defaultHookDir, "libalpm hooks directory")
	cmd.Flags().StringVar(&configPath, "config-path", "", "kayo config path to bake into the hook's Exec")
	return cmd
}

func hookUninstallCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the installed pacman hook",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := filepath.Join(dir, hookFileName)
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", path)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", defaultHookDir, "libalpm hooks directory")
	return cmd
}
