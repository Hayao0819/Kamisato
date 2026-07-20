// Package hookcmd builds the shared "install"/"uninstall" pacman-hook cobra
// commands for ayaka and kayo. Only the hook file name, template, and baked Exec
// line differ, so the scaffolding lives here once.
package hookcmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/pacman"
	"github.com/Hayao0819/Kamisato/pkg/pacman/hook"
)

// InstallOptions parameterizes NewInstallCmd for one app's hook.
type InstallOptions struct {
	// BinName names the app in error messages ("ayaka", "kayo").
	BinName string
	// FileName is the installed hook's file name; Template is its embedded body.
	FileName string
	Template string
	// SetupFlags registers the app-specific flags; --dir and --pacman-config are
	// always registered.
	SetupFlags func(*cobra.Command)
	// BuildExec turns the resolved absolute path of the running binary into the
	// hook's Exec line. It receives the parsed --pacman-config for apps that bake
	// it in, runs after flags are parsed, and must validate every baked value
	// with hook.ValidateExecArg.
	BuildExec func(self, pacmanConf string) (string, error)
}

// NewInstallCmd builds the "install" subcommand: it resolves the running binary,
// defaults the hook dir from pacman.conf, and writes the templated hook.
func NewInstallCmd(opts InstallOptions) *cobra.Command {
	var dir, pacmanConf string
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the pacman hook (writes to a system dir; usually needs root)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			self, err := os.Executable()
			if err != nil {
				return errors.WrapErr(err, "cannot resolve the "+opts.BinName+" binary path")
			}
			if err := hook.ValidateExecArg(opts.BinName+" binary path", self); err != nil {
				return err
			}
			exec, err := opts.BuildExec(self, pacmanConf)
			if err != nil {
				return err
			}
			if dir == "" {
				dir = pacman.HookDir(pacmanConf)
			}
			path, err := hook.Install(dir, opts.FileName, opts.Template, exec)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "installed %s\n", path)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "hook directory (default: pacman.conf HookDir)")
	cmd.Flags().StringVar(&pacmanConf, "pacman-config", "", "pacman.conf path for resolving HookDir")
	if opts.SetupFlags != nil {
		opts.SetupFlags(cmd)
	}
	return cmd
}

// NewUninstallCmd builds the "uninstall" subcommand that removes the named hook.
func NewUninstallCmd(fileName string) *cobra.Command {
	var dir, pacmanConf string
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the installed pacman hook",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dir == "" {
				dir = pacman.HookDir(pacmanConf)
			}
			path, err := hook.Uninstall(dir, fileName)
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
