package statuscmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/ayaka/service/report"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "status [<srcrepo>]",
		Short:             "Show build status of packages in a source repository (.ayakarc)",
		Long:              "Show, like git status, which source packages in a source repository (.ayakarc) failed to build, are out of date (PKGBUILD ahead of the published package), are building, or were never published.",
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
			// report.DefaultListFormat references every column, so all are populated.
			rows := report.BuildRows(repos, report.DefaultListFormat, report.FetchJobsBestEffort(server))
			report.PrintStatus(cmd.OutOrStdout(), rows, len(repos) > 1)
			return nil
		},
	}

	shared.AddServerFlag(cmd)
	return cmd
}
