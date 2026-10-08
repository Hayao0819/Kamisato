package pullcmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/completion"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/sourcerepos"
	"github.com/Hayao0819/Kamisato/ayaka/service/source"
	sourcerepo "github.com/Hayao0819/Kamisato/ayaka/source"
	"github.com/Hayao0819/Kamisato/internal/pacman/pkg"
	"github.com/spf13/cobra"
)

// pullFunc is the source-update operation this command drives.
type pullFunc func(ctx context.Context, src *sourcerepo.SourceRepo, names []string, force bool) ([]*pkg.SourcePackage, error)

// Options groups the repository and mirror targets of a pull operation.
type Options struct {
	Repository string
	Packages   []string
	Force      bool
}

func Cmd() *cobra.Command {
	return newCommand(source.PullPackages, nil, nil)
}

func newCommand(pull pullFunc, resolve sourcerepos.LookupFunc, complete cobra.CompletionFunc) *cobra.Command {
	var options Options
	var cmd cobra.Command
	if resolve == nil {
		sources := sourcerepos.ForCommand(&cmd)
		resolve = sources.Find
		if complete == nil {
			complete = completion.CompleteSrcRepoThenPackages(sources)
		}
	}
	cmd = cobra.Command{
		Use:               "pull <srcrepo> [pkgname...]",
		Short:             "Sync mirror packages with their origin (all mirrors when no name is given)",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: complete,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.Repository = args[0]
			options.Packages = args[1:]
			return run(cmd.Context(), cmd.OutOrStdout(), resolve, pull, options)
		},
	}
	cmd.Flags().BoolVarP(&options.Force, "force", "f", false, "Discard local changes in the checkout before syncing")
	return &cmd
}

func run(ctx context.Context, out io.Writer, resolve sourcerepos.LookupFunc, pull pullFunc, options Options) error {
	repo, err := sourcerepos.Require(resolve, options.Repository)
	if err != nil {
		return err
	}
	pulled, err := pull(ctx, repo, options.Packages, options.Force)
	for _, pkg := range pulled {
		slog.Info("pulled", "pkgbase", pkg.Base(), "version", pkg.Version())
	}
	if err != nil {
		return err
	}
	if len(pulled) == 0 {
		fmt.Fprintln(out, "no mirror packages to pull")
	}
	return nil
}
