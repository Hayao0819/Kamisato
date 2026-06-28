package aurcmd

import (
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/spf13/cobra"
)

// aurAddCmd clones one or more AUR packages into a source repository for the
// first time. When the package is already present it falls back to a pull, so
// the command is safe to re-run.
func aurAddCmd() *cobra.Command {
	var force bool
	const aurBase = "https://aur.archlinux.org"

	cmd := &cobra.Command{
		Use:   "add <repo> <aur-pkg> [aur-pkg...]",
		Short: "Add AUR packages to a source repository",
		Args:  cobra.MinimumNArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return shared.GetSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
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
