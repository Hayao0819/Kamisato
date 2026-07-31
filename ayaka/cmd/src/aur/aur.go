package aurcmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/ayaka/service/source"
)

// aurManager is the slice of service/source this command drives.
type aurManager interface {
	Add(ctx context.Context, repoDir string, names []string, force bool) error
}

type sourceAurManager struct{}

func (sourceAurManager) Add(ctx context.Context, repoDir string, names []string, force bool) error {
	return source.AddAUR(ctx, repoDir, names, force)
}

func Cmd(runtime *app.Runtime) *cobra.Command { return newCommand(sourceAurManager{}, runtime) }

func newCommand(svc aurManager, runtime *app.Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aur",
		Short: "Manage PKGBUILDs taken from the AUR",
		Long:  "Add AUR packages to a source repository; keep them current with 'ayaka src pull'.",
	}
	cmd.AddCommand(
		aurAddCmd(svc, runtime),
	)
	return cmd
}
