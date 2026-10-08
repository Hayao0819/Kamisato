package srcinfocmd

import (
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/completion"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/sourcerepos"
	"github.com/Hayao0819/Kamisato/ayaka/service/source"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/spf13/cobra"
)

// Cmd regenerates .SRCINFO files; with no argument it covers every configured repository.
func newCommand(sources sourcerepos.Reader) *cobra.Command {
	cmd := &cobra.Command{}
	if sources == nil {
		sources = sourcerepos.ForCommand(cmd)
	}
	*cmd = cobra.Command{
		Use:               "srcinfo [<srcrepo>]",
		Aliases:           []string{"us"},
		Short:             "Regenerate .SRCINFO files in a source repository (.ayakarc)",
		Long:              "Regenerate .SRCINFO files for the source packages in a source repository (.ayakarc).",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completion.CompleteSrcRepoNames(sources),
		RunE: func(cmd *cobra.Command, args []string) error {
			repos, err := sourcerepos.Select(sources, args)
			if err != nil {
				return err
			}
			for _, repo := range repos {
				if repo.Dir == "" {
					return errors.WrapErr(sourcerepos.ErrSourceRepoNotFound, repo.Config.Name)
				}
				dir := repo.Dir
				if err := source.RegenerateSrcinfoStrict(cmd.Context(), dir, cmd.ErrOrStderr(), func(d string) {
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

func Cmd() *cobra.Command { return newCommand(nil) }
