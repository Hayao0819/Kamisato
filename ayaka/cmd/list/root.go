package listcmd

import (
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/spf13/cobra"
)

// Cmd lists source packages with their versions and build status; columns are
// selectable with a Docker-style --format template.
func Cmd() *cobra.Command {
	var format string
	var server string

	cmd := cobra.Command{
		Use:   "list [repo]",
		Short: "List source packages with their versions and build status",
		Long: "List source packages as a table.\n\n" +
			"Columns are chosen with a Go template via --format, like docker:\n" +
			"  ayaka list --format 'table {{.Package}}\\t{{.Local}}\\t{{.Remote}}'\n" +
			"  ayaka list --format '{{.Package}} {{.Build}}'\n" +
			"  ayaka list --format json\n\n" +
			"Fields: .Package .Installed .Local .Remote .Build",
		Args: cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return shared.AppFrom(cmd).GetSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			app := shared.AppFrom(cmd)
			repos := app.SrcRepos
			if len(args) > 0 {
				argrepo := app.GetSrcRepo(args[0])
				if argrepo == nil {
					return utils.WrapErr(shared.ErrInvalidRepoName, args[0])
				}
				repos = []*repo.SourceRepo{argrepo}
			}

			if format == "" {
				format = shared.DefaultListFormat
			}
			rows := shared.BuildPkgRows(repos, format, server)
			return renderRows(cmd.OutOrStdout(), format, rows)
		},
	}

	cmd.Flags().StringVar(&format, "format", "", "Format the output with a Go template (Docker-style; 'table ...' or 'json')")
	cmd.Flags().StringVarP(&server, "server", "s", "", "ayato server for build status (default: serverdb default)")
	return &cmd
}
