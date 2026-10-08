package listcmd

import (
	"context"
	"io"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/completion"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/remote"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/sourcerepos"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/src/internal/report"
	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	"github.com/spf13/cobra"
)

var pkgHeader = report.Row{
	Repo:      "REPO",
	Package:   "PACKAGE",
	Installed: "INSTALLED",
	Local:     "LOCAL",
	Remote:    "REMOTE",
	Build:     "BUILD",
}

func renderRows(out io.Writer, format string, rows []report.Row) error {
	return cmdline.RenderList(out, format, pkgHeader, rows)
}

// Cmd lists source packages with their versions and build status; columns are
// selectable with a Docker-style --format template.
func newCommand(sources sourcerepos.Reader) *cobra.Command {
	var cmd cobra.Command
	if sources == nil {
		sources = sourcerepos.ForCommand(&cmd)
	}
	cmd = cobra.Command{
		Use:   "list [<srcrepo>]",
		Short: "List source packages in a source repository (.ayakarc)",
		Long: "List source packages as a table.\n\n" +
			"Columns are chosen with a Go template via --format, like docker:\n" +
			"  ayaka src list --format 'table {{.Package}}\\t{{.Local}}\\t{{.Remote}}'\n" +
			"  ayaka src list --format '{{.Package}} {{.Build}}'\n" +
			"  ayaka src list --format json\n\n" +
			"Fields: .Package .Installed .Local .Remote .Build",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completion.CompleteSrcRepoNames(sources),
		RunE: func(cmd *cobra.Command, args []string) error {
			server, err := cmd.Flags().GetString("server")
			if err != nil {
				return err
			}
			format, err := cmdline.ResolveFormat(cmd, report.DefaultListFormat)
			if err != nil {
				return err
			}
			return run(cmd.Context(), cmd.OutOrStdout(), sources, Options{Repositories: args, Server: server, Format: format})
		},
	}

	cmdline.AddFormatFlags(&cmd)
	remote.AddServerFlag(&cmd)
	return &cmd
}

// Options is the parsed input to this command, independent of Cobra.
type Options struct {
	Repositories []string
	Server       string
	Format       string
}

func run(ctx context.Context, out io.Writer, sources sourcerepos.Reader, options Options) error {
	repos, err := sourcerepos.Select(sources, options.Repositories)
	if err != nil {
		return err
	}

	rows := report.BuildRows(ctx, remote.DatabaseClient(), repos, options.Format, report.FetchJobsBestEffort(ctx, options.Server))
	return renderRows(out, options.Format, rows)
}

func Cmd() *cobra.Command { return newCommand(nil) }
