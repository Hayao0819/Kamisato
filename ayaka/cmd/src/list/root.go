package listcmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/ayaka/service/report"
	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// Cmd lists source packages with their versions and build status; columns are
// selectable with a Docker-style --format template.
func Cmd() *cobra.Command {
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
		ValidArgsFunction: shared.CompleteSrcRepoNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			a := app.From(cmd)
			repos := a.SrcRepos
			if len(args) > 0 {
				argrepo := a.GetSrcRepo(args[0])
				if argrepo == nil {
					return errors.WrapErr(shared.ErrSourceRepoNotFound, args[0])
				}
				repos = []*repo.SourceRepo{argrepo}
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
	shared.AddServerFlag(&cmd)
	return &cmd
}
