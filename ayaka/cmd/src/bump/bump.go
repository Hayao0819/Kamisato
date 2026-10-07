package bumpcmd

import (
	"io"
	"log/slog"
	"strings"

	"github.com/samber/lo"
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/ayaka/cli"
	"github.com/Hayao0819/Kamisato/ayaka/service/source"
	"github.com/Hayao0819/Kamisato/internal/errors"
	pkg "github.com/Hayao0819/Kamisato/internal/pacman/pkg"
	pacmansource "github.com/Hayao0819/Kamisato/internal/pacman/source"
)

// bumper is the slice of service/source this command drives.
type bumper interface {
	Bump(src *pacmansource.SourceRepo, names []string, by string, stderr io.Writer) ([]*pkg.SourcePackage, error)
	Commit(srcDir string, bumped []*pkg.SourcePackage, message string) (string, error)
}

type sourceBumper struct{}

func (sourceBumper) Bump(src *pacmansource.SourceRepo, names []string, by string, stderr io.Writer) ([]*pkg.SourcePackage, error) {
	return source.BumpPkgrel(src, names, by, stderr)
}

func (sourceBumper) Commit(srcDir string, bumped []*pkg.SourcePackage, message string) (string, error) {
	return source.CommitBump(srcDir, bumped, message)
}

// Cmd raises pkgrel for packages that must rebuild without a source change of
// their own (a dependency update); source edits stay on the ayaka side so ayato
// never touches sources.
func Cmd(runtime *app.Runtime) *cobra.Command { return newCommand(sourceBumper{}, runtime) }

func newCommand(svc bumper, runtime *app.Runtime) *cobra.Command {
	var by string
	var message string
	var noCommit bool
	cmd := cobra.Command{
		Use:               "bump <srcrepo> <pkgname>...",
		Short:             "Raise pkgrel for a rebuild and commit the change",
		Args:              cobra.MinimumNArgs(2),
		ValidArgsFunction: cli.CompleteSrcRepoThenPackages(runtime),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := runtime.App()
			if err != nil {
				return err
			}
			srcrepo := a.GetSrcRepo(args[0])
			if srcrepo == nil {
				return errors.WrapErr(cli.ErrSourceRepoNotFound, args[0])
			}

			bumped, err := svc.Bump(srcrepo, args[1:], by, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			for _, p := range bumped {
				slog.Info("bumped pkgrel", "pkgbase", p.Base(), "version", p.Version())
			}
			if noCommit {
				return nil
			}

			bases := lo.Map(bumped, func(p *pkg.SourcePackage, _ int) string { return p.Base() })
			if message == "" {
				message = "chore: rebuild " + strings.Join(bases, " ") + " for dependency update"
			}
			hash, err := svc.Commit(srcrepo.Dir, bumped, message)
			if err != nil {
				return err
			}
			slog.Info("committed pkgrel bump", "commit", hash, "packages", bases)
			return nil
		},
	}
	cmd.Flags().StringVar(&by, "by", "0.1", "Pkgrel step: 0.1 (1 -> 1.1) or 1 (next integer)")
	cmd.Flags().StringVar(&message, "message", "", "Commit message (default: a rebuild message naming the packages)")
	cmd.Flags().BoolVar(&noCommit, "no-commit", false, "Edit PKGBUILD/.SRCINFO only; leave committing to the caller")
	return &cmd
}
