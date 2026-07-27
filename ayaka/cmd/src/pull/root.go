package pullcmd

import (
	"context"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/ayaka/service/source"
	"github.com/Hayao0819/Kamisato/internal/errors"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// puller is the slice of service/source this command drives.
type puller interface {
	Pull(ctx context.Context, src *repo.SourceRepo, names []string, force bool) ([]*pkg.SourcePackage, error)
}

type sourcePuller struct{}

func (sourcePuller) Pull(ctx context.Context, src *repo.SourceRepo, names []string, force bool) ([]*pkg.SourcePackage, error) {
	return source.PullPackages(ctx, src, names, force)
}

// Cmd advances mirror packages (dirs that are their own git checkout, like AUR
// submodules) to their origin HEAD. It is the one implementation of that
// operation: `aur update` and `submodules --remote` folded into it.
func Cmd() *cobra.Command { return newCommand(sourcePuller{}) }

func newCommand(svc puller) *cobra.Command {
	var force bool
	cmd := cobra.Command{
		Use:               "pull <srcrepo> [pkgname...]",
		Short:             "Sync mirror packages with their origin (all mirrors when no name is given)",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: shared.CompleteSrcRepoThenPackages,
		RunE: func(cmd *cobra.Command, args []string) error {
			srcrepo := app.From(cmd).GetSrcRepo(args[0])
			if srcrepo == nil {
				return errors.WrapErr(shared.ErrSourceRepoNotFound, args[0])
			}

			pulled, err := svc.Pull(cmd.Context(), srcrepo, args[1:], force)
			for _, p := range pulled {
				slog.Info("pulled", "pkgbase", p.Base(), "version", p.Version())
			}
			if err != nil {
				return err
			}
			if len(pulled) == 0 {
				cmd.Println("no mirror packages to pull")
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Discard local changes in the checkout before syncing")
	return &cmd
}
