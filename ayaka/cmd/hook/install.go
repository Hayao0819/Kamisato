package hookcmd

import (
	"fmt"
	"os"

	"github.com/Hayao0819/Kamisato/internal/pacmanhook"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/spf13/cobra"
)

func hookInstallCmd() *cobra.Command {
	var dir, repo, server, pacmanConf string
	var buildDirs []string
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the pacman hook (writes to a system dir; usually needs root)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if repo == "" {
				return utils.NewErr("--repo is required")
			}
			self, err := os.Executable()
			if err != nil {
				return utils.WrapErr(err, "cannot resolve the ayaka binary path")
			}
			// These are baked bare into the hook's Exec line; reject values that
			// would word-split into injected flags (e.g. a repo of "x --all").
			toBake := []struct{ name, val string }{{"ayaka binary path", self}, {"--repo", repo}, {"--server", server}, {"--pacman-config", pacmanConf}}
			for _, d := range buildDirs {
				toBake = append(toBake, struct{ name, val string }{"--build-dir", d})
			}
			for _, a := range toBake {
				if err := pacmanhook.ValidateExecArg(a.name, a.val); err != nil {
					return err
				}
			}
			// The hook runs as root under pacman, so credentials resolve against
			// root's server database at runtime, not the installing user's. Only
			// the repo (and optional server/config/build-dir) are baked in — never a secret.
			execLine := self + " hook upload --repo " + repo
			if server != "" {
				execLine += " --server " + server
			}
			if pacmanConf != "" {
				execLine += " --pacman-config " + pacmanConf
			}
			for _, d := range buildDirs {
				execLine += " --build-dir " + d
			}
			if dir == "" {
				dir = pacmanhook.HookDir(pacmanConf)
			}
			path, err := pacmanhook.Install(dir, uploadHookFileName, uploadHookTemplate, execLine)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "installed %s\n", path)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "hook directory (default: pacman.conf HookDir)")
	cmd.Flags().StringVar(&repo, "repo", "", "target repository on ayato (required)")
	cmd.Flags().StringVar(&server, "server", "", "ayato server to bake in (default: server db default at runtime)")
	cmd.Flags().StringVar(&pacmanConf, "pacman-config", "", "pacman.conf path for resolving HookDir, and baked in for CacheDir resolution")
	cmd.Flags().StringArrayVar(&buildDirs, "build-dir", nil, "dir(s) holding locally-built packages (e.g. makepkg PKGDEST), baked into the hook")
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
