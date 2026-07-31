package aurcmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/ayaka/cli"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

func aurAddCmd(svc aurManager, runtime *app.Runtime) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:               "add <srcrepo> <pkgname>...",
		Short:             "Clone AUR packages into a source repository (.ayakarc)",
		Args:              cobra.MinimumNArgs(2),
		ValidArgsFunction: cli.CompleteSrcRepoThenPackages(runtime),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := runtime.App()
			if err != nil {
				return err
			}
			repo := a.GetSrcRepo(args[0])
			if repo == nil {
				return errors.WrapErr(cli.ErrSourceRepoNotFound, args[0])
			}
			if repo.Dir == "" {
				return errors.WrapErr(cli.ErrNoSourceDir, args[0])
			}
			return svc.Add(cmd.Context(), repo.Dir, args[1:], force)
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Re-clone even if already tracked")
	cmd.ValidArgsFunction = cli.CompleteSrcRepoNames(runtime)
	return cmd
}
