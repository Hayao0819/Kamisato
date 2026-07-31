package repocmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/ayaka/cli"
	prunecmd "github.com/Hayao0819/Kamisato/ayaka/cmd/repo/prune"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

func repoRemoveCmd(runtime *app.Runtime) *cobra.Command {
	var diffMode bool
	var arch string
	var dryRun bool
	var diffURL string
	cmd := &cobra.Command{
		Use:   "remove <repo> <pkgname>... | --diff <srcrepo>",
		Short: "Remove packages by name, or (--diff) prune ones no longer in a source repo",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if diffMode {
				return prunecmd.Run(cmd, runtime, args[0], arch, diffURL, dryRun)
			}
			if len(args) < 2 {
				return errors.NewErr("remove needs <repo> <pkgname>... (or --diff <srcrepo>)")
			}
			client, err := cli.RepoClient(cmd)
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
	cli.AddRepoServerFlags(cmd)
	cmd.Flags().BoolVar(&diffMode, "diff", false, "Prune packages no longer present in the source repo (arg is the source repo)")
	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Architecture whose remote db defines the current package set (with --diff)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "With --diff, list what would be removed without deleting")
	cmd.Flags().StringVar(&diffURL, "diff-url", "", "Remote repo db dir for --diff (.../repo/<repo>/<arch>); overrides repo.json url")
	return cmd
}
