package aurcmd

import "github.com/spf13/cobra"

func aurUpdateCmd(svc aurManager) *cobra.Command {
	return aurMutationCmd(
		"update <srcrepo> <pkgname>...",
		"Pull tracked AUR packages from upstream",
		"Force pull even on unclean state",
		func(cmd *cobra.Command, repoName string, pkgs []string, force bool) error {
			dir, err := repoDir(cmd, repoName)
			if err != nil {
				return err
			}
			return svc.Update(cmd.Context(), dir, pkgs, force)
		},
	)
}
