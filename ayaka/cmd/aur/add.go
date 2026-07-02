package aurcmd

import (
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/spf13/cobra"
)

// aurAddCmd clones AUR packages into a source repository, falling back to a pull
// when one is already present so it is safe to re-run.
func aurAddCmd() *cobra.Command {
	var force bool
	const aurBase = "https://aur.archlinux.org"

	cmd := &cobra.Command{
		Use:   "add <repo> <aur-pkg> [aur-pkg...]",
		Short: "Add AUR packages to a source repository",
		Args:  cobra.MinimumNArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return shared.AppFrom(cmd).GetSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAurFetch(cmd, args[0], args[1:], aurBase, force)
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Re-clone repositories even if they already exist")
	return cmd
}
