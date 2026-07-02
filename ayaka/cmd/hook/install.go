package hookcmd

import (
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	sharedhook "github.com/Hayao0819/Kamisato/internal/hookcmd"
	"github.com/Hayao0819/Kamisato/pkg/pacman/hook"
	"github.com/spf13/cobra"
)

func hookInstallCmd() *cobra.Command {
	var repo, server string
	var buildDirs []string
	return sharedhook.NewInstallCmd(sharedhook.InstallOptions{
		BinName:  "ayaka",
		FileName: uploadHookFileName,
		Template: uploadHookTemplate,
		SetupFlags: func(cmd *cobra.Command) {
			cmd.Flags().StringVar(&repo, "repo", "", "target repository on ayato (required)")
			cmd.Flags().StringVar(&server, "server", "", "ayato server to bake in (default: server db default at runtime)")
			cmd.Flags().StringArrayVar(&buildDirs, "build-dir", nil, "dir(s) holding locally-built packages (e.g. makepkg PKGDEST), baked into the hook")
		},
		BuildExec: func(self, pacmanConf string) (string, error) {
			if repo == "" {
				return "", errwrap.NewErr("--repo is required")
			}
			// These are baked bare into the hook's Exec line; reject values that
			// would word-split into injected flags (e.g. a repo of "x --all").
			toBake := []struct{ name, val string }{{"--repo", repo}, {"--server", server}, {"--pacman-config", pacmanConf}}
			for _, d := range buildDirs {
				toBake = append(toBake, struct{ name, val string }{"--build-dir", d})
			}
			for _, a := range toBake {
				if err := hook.ValidateExecArg(a.name, a.val); err != nil {
					return "", err
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
			return execLine, nil
		},
	})
}
