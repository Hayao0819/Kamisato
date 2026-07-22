package aurcmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/ayaka/service/source"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

// aurManager is the slice of service/source this command drives.
type aurManager interface {
	Add(ctx context.Context, repoDir string, names []string, force bool) error
	Update(ctx context.Context, repoDir string, names []string, force bool) error
}

type sourceAurManager struct{}

func (sourceAurManager) Add(ctx context.Context, repoDir string, names []string, force bool) error {
	return source.AddAUR(ctx, repoDir, names, force)
}

func (sourceAurManager) Update(ctx context.Context, repoDir string, names []string, force bool) error {
	return source.UpdateAUR(ctx, repoDir, names, force)
}

func Cmd() *cobra.Command { return newCommand(sourceAurManager{}) }

func newCommand(svc aurManager) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aur",
		Short: "Manage PKGBUILDs taken from the AUR",
		Long:  "Add AUR packages to a source repository and update them from upstream.",
	}
	cmd.AddCommand(
		aurAddCmd(svc),
		aurUpdateCmd(svc),
	)
	return cmd
}

func aurMutationCmd(
	use, short, forceHelp string,
	run func(*cobra.Command, string, []string, bool) error,
) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:               use,
		Short:             short,
		Args:              cobra.MinimumNArgs(2),
		ValidArgsFunction: shared.CompleteSrcRepoThenPackages,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, args[0], args[1:], force)
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, forceHelp)
	return cmd
}

func repoDir(cmd *cobra.Command, repoName string) (string, error) {
	repo := app.From(cmd).GetSrcRepo(repoName)
	if repo == nil {
		return "", errors.WrapErr(shared.ErrSourceRepoNotFound, repoName)
	}
	if repo.Dir == "" {
		return "", errors.WrapErr(shared.ErrNoSourceDir, repoName)
	}
	return repo.Dir, nil
}
