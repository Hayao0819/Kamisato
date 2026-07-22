package srcinfocmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/ayaka/service/source"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

// Cmd regenerates .SRCINFO files; with no argument it covers every configured repository.
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "srcinfo [<srcrepo>]",
		Aliases:           []string{"us"},
		Short:             "Regenerate .SRCINFO files in a source repository (.ayakarc)",
		Long:              "Regenerate .SRCINFO files for the source packages in a source repository (.ayakarc).",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: shared.CompleteSrcRepoNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			a := app.From(cmd)
			dirs := make([]string, 0, len(a.Config.Repos))
			if len(args) > 0 {
				sourceRepo := a.GetSrcRepo(args[0])
				if sourceRepo == nil || sourceRepo.Dir == "" {
					return errors.WrapErr(shared.ErrSourceRepoNotFound, args[0])
				}
				dirs = append(dirs, sourceRepo.Dir)
			} else {
				for _, r := range a.Config.Repos {
					dirs = append(dirs, r.Dir)
				}
			}

			for _, dir := range dirs {
				if err := source.RegenerateSrcinfoStrict(dir, cmd.ErrOrStderr(), func(d string) {
					cmd.Println("Updated SRCINFO file:", d)
				}); err != nil {
					return err
				}
			}
			return nil
		},
	}

	return cmd
}
