package listcmd

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/ayaka/cli"
	"github.com/Hayao0819/Kamisato/ayaka/service/report"
	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/pacman/source"
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
	return cliutil.RenderList(out, format, pkgHeader, rows)
}

// Cmd lists source packages with their versions and build status; columns are
// selectable with a Docker-style --format template.
func Cmd(runtime *app.Runtime) *cobra.Command {
	cmd := cobra.Command{
		Use:   "list [<srcrepo>]",
		Short: "List source packages in a source repository (.ayakarc)",
		Long: "List source packages as a table.\n\n" +
			"Columns are chosen with a Go template via --format, like docker:\n" +
			"  ayaka list --format 'table {{.Package}}\\t{{.Local}}\\t{{.Remote}}'\n" +
			"  ayaka list --format '{{.Package}} {{.Build}}'\n" +
			"  ayaka list --format json\n\n" +
			"Fields: .Package .Installed .Local .Remote .Build",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cli.CompleteSrcRepoNames(runtime),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := runtime.App()
			if err != nil {
				return err
			}
			repos := a.SrcRepos
			if len(args) > 0 {
				argrepo := a.GetSrcRepo(args[0])
				if argrepo == nil {
					return errors.WrapErr(cli.ErrSourceRepoNotFound, args[0])
				}
				repos = []*source.SourceRepo{argrepo}
			}

			server, err := cmd.Flags().GetString("server")
			if err != nil {
				return err
			}
			format, err := cliutil.ResolveFormat(cmd, report.DefaultListFormat)
			if err != nil {
				return err
			}
			rows := report.BuildRows(repos, format, report.FetchJobsBestEffort(server))
			return renderRows(cmd.OutOrStdout(), format, rows)
		},
	}

	cliutil.AddFormatFlags(&cmd)
	cli.AddServerFlag(&cmd)
	return &cmd
}
