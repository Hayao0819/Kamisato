package removecmd

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/remote"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/sourcerepos"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/repo/internal/pruning"
	"github.com/Hayao0819/Kamisato/ayaka/service/plan"
	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	"github.com/spf13/cobra"
)

func newCommand(sources sourcerepos.Reader) *cobra.Command {
	var diffMode bool
	var arch string
	var dryRun bool
	var diffURL string
	cmd := &cobra.Command{}
	if sources == nil {
		sources = sourcerepos.ForCommand(cmd)
	}
	*cmd = cobra.Command{
		Use:   "remove <repo> <pkgname>... | --diff <srcrepo>",
		Short: "Remove packages by name, or (--diff) prune ones no longer in a source repo",
		Args: func(cmd *cobra.Command, args []string) error {
			if diffMode {
				return cobra.ExactArgs(1)(cmd, args)
			}
			for _, name := range []string{"dry-run", "arch", "diff-url"} {
				if cmd.Flags().Changed(name) {
					return &cmdline.UsageError{Err: fmt.Errorf("--%s requires --diff", name)}
				}
			}
			return cobra.MinimumNArgs(2)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if diffMode {
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
			}
			client, err := remote.DefaultProvider().RepoClient(cmd)
			if err != nil {
				return err
			}
			for _, name := range args[1:] {
				if err := client.RemovePackageAllArchitectures(cmd.Context(), args[0], name); err != nil {
					return err
				}
			}
			return nil
		},
	}
	remote.AddRepoServerFlags(cmd)
	cmd.Flags().BoolVar(&diffMode, "diff", false, "Prune packages no longer present in the source repo (arg is the source repo)")
	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Architecture whose remote db defines the current package set (with --diff)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "With --diff, list what would be removed without deleting")
	cmd.Flags().StringVar(&diffURL, "diff-url", "", "Remote repo db dir for --diff (.../repo/<repo>/<arch>); overrides repo.json url")
	return cmd
}

func Cmd() *cobra.Command { return newCommand(nil) }
