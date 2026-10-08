package prunecmd

import (
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/completion"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/remote"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/sourcerepos"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/repo/internal/pruning"
	"github.com/Hayao0819/Kamisato/ayaka/service/plan"
	"github.com/spf13/cobra"
)

// Cmd is the promoted top-level form of `repo remove --diff`: it deletes from
// ayato what the source repo no longer provides.
func newCommand(sources sourcerepos.Reader) *cobra.Command {
	var arch string
	var dryRun bool
	var diffURL string
	var cmd cobra.Command
	if sources == nil {
		sources = sourcerepos.ForCommand(&cmd)
	}
	cmd = cobra.Command{
		Use:               "prune <srcrepo>",
		Short:             "Remove packages from ayato that are no longer in the source repo",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.CompleteSrcRepoNames(sources),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := sourcerepos.Require(sources.Find, args[0])
			if err != nil {
				return err
			}
			remove, err := remote.DefaultProvider().Removal(cmd)
			if err != nil {
				return err
			}
			return pruning.Run(cmd.Context(), cmd.OutOrStdout(), remote.DatabaseClient(), src, plan.PruneOptions{
				Arch: arch, DatabaseURL: diffURL, DryRun: dryRun, Remove: remove,
			})
		},
	}
	remote.AddRepoServerFlags(&cmd)
	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Architecture whose remote db defines the current package set")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "List what would be removed without deleting")
	cmd.Flags().StringVar(&diffURL, "diff-url", "", "Remote repo db dir (.../repo/<repo>/<arch>); overrides repo.json url")
	return &cmd
}

func Cmd() *cobra.Command { return newCommand(nil) }
