package prunecmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/ayaka/cli"
	"github.com/Hayao0819/Kamisato/ayaka/service/plan"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

// Cmd is the promoted top-level form of `repo remove --diff`: it deletes from
// ayato what the source repo no longer provides.
func Cmd(runtime *app.Runtime) *cobra.Command {
	var arch string
	var dryRun bool
	var diffURL string
	cmd := cobra.Command{
		Use:               "prune <srcrepo>",
		Short:             "Remove packages from ayato that are no longer in the source repo",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cli.CompleteSrcRepoNames(runtime),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(cmd, runtime, args[0], arch, diffURL, dryRun)
		},
	}
	cli.AddRepoServerFlags(&cmd)
	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Architecture whose remote db defines the current package set")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "List what would be removed without deleting")
	cmd.Flags().StringVar(&diffURL, "diff-url", "", "Remote repo db dir (.../repo/<repo>/<arch>); overrides repo.json url")
	return &cmd
}

func Run(cmd *cobra.Command, runtime *app.Runtime, sourceName, arch, databaseURL string, dryRun bool) error {
	a, err := runtime.App()
	if err != nil {
		return err
	}
	src := a.GetSrcRepo(sourceName)
	if src == nil {
		return errors.WrapErr(cli.ErrSourceRepoNotFound, sourceName)
	}

	var client interface {
		RemovePackageAllArchitectures(context.Context, string, string) error
	}
	packages, err := plan.Prune(cmd.Context(), src, plan.PruneOptions{
		Arch:        arch,
		DatabaseURL: databaseURL,
		DryRun:      dryRun,
		Remove: func(ctx context.Context, repository, name string) error {
			if client == nil {
				client, err = cli.RepoClient(cmd)
				if err != nil {
					return err
				}
			}
			return client.RemovePackageAllArchitectures(ctx, repository, name)
		},
	})
	if err != nil {
		return err
	}
	if dryRun {
		for _, name := range packages {
			fmt.Fprintln(cmd.OutOrStdout(), name)
		}
	}
	return nil
}
