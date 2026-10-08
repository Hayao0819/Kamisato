package statuscmd

import (
	"context"
	"io"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/completion"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/remote"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/sourcerepos"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/src/internal/report"
	"github.com/spf13/cobra"
)

func newCommand(sources sourcerepos.Reader) *cobra.Command {
	cmd := &cobra.Command{}
	if sources == nil {
		sources = sourcerepos.ForCommand(cmd)
	}
	*cmd = cobra.Command{
		Use:               "status [<srcrepo>]",
		Short:             "Show build status of packages in a source repository (.ayakarc)",
		Long:              "Show, like git status, which source packages in a source repository (.ayakarc) failed to build, are out of date (PKGBUILD ahead of the published package), are building, or were never published.",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completion.CompleteSrcRepoNames(sources),
		RunE: func(cmd *cobra.Command, args []string) error {
			server, err := cmd.Flags().GetString("server")
			if err != nil {
				return err
			}
			return run(cmd.Context(), cmd.OutOrStdout(), sources, Options{Repositories: args, Server: server})
		},
	}

	remote.AddServerFlag(cmd)
	return cmd
}

// Options is the parsed input to this command, independent of Cobra.
type Options struct {
	Repositories []string
	Server       string
}

func run(ctx context.Context, out io.Writer, sources sourcerepos.Reader, options Options) error {
	repos, err := sourcerepos.Select(sources, options.Repositories)
	if err != nil {
		return err
	}

	// report.DefaultListFormat references every column, so all are populated.
	rows := report.BuildRows(ctx, remote.DatabaseClient(), repos, report.DefaultListFormat, report.FetchJobsBestEffort(ctx, options.Server))
	report.PrintStatus(out, rows, len(repos) > 1)
	return nil
}

func Cmd() *cobra.Command { return newCommand(nil) }
