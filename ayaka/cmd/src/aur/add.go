package aurcmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
)

func aurAddCmd(svc aurManager) *cobra.Command {
	cmd := aurMutationCmd(
		"add <srcrepo> <pkgname>...",
		"Clone AUR packages into a source repository (.ayakarc)",
		"Re-clone even if already tracked",
		func(cmd *cobra.Command, repoName string, pkgs []string, force bool) error {
			dir, err := repoDir(cmd, repoName)
			if err != nil {
				return err
			}
			return svc.Add(cmd.Context(), dir, pkgs, force)
		},
	)
	// New AUR packages are not local yet, so tracked-package completion would
	// only mislead here.
	cmd.ValidArgsFunction = shared.CompleteSrcRepoNames
	return cmd
}
