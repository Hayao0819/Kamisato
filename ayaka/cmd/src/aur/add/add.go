package addcmd

import (
	"context"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/completion"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/sourcerepos"
	"github.com/Hayao0819/Kamisato/ayaka/service/source"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/spf13/cobra"
)

func newCommand(add func(context.Context, string, []string, bool) error, sources sourcerepos.Reader) *cobra.Command {
	var force bool
	cmd := &cobra.Command{}
	if sources == nil {
		sources = sourcerepos.ForCommand(cmd)
	}
	*cmd = cobra.Command{
		Use:               "add <srcrepo> <pkgname>...",
		Short:             "Clone AUR packages into a source repository (.ayakarc)",
		Args:              cobra.MinimumNArgs(2),
		ValidArgsFunction: completion.CompleteSrcRepoNames(sources),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := sourcerepos.Require(sources.Find, args[0])
			if err != nil {
				return err
			}
			if repo.Dir == "" {
				return errors.WrapErr(sourcerepos.ErrNoSourceDir, args[0])
			}
			return add(cmd.Context(), repo.Dir, args[1:], force)
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Re-clone even if already tracked")
	return cmd
}

func Cmd() *cobra.Command { return newCommand(source.AddAUR, nil) }
