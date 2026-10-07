package nvbumpcmd

import (
	"io"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/ayaka/cli"
	"github.com/Hayao0819/Kamisato/ayaka/service/source"
	"github.com/Hayao0819/Kamisato/internal/errors"
	pkg "github.com/Hayao0819/Kamisato/internal/pacman/pkg"
	pacmansource "github.com/Hayao0819/Kamisato/internal/pacman/source"
)

// nvBumper is the slice of service/source this command drives.
type nvBumper interface {
	NvBump(src *pacmansource.SourceRepo, name, newVersion string, stderr io.Writer) (*pkg.SourcePackage, error)
	Commit(srcDir string, bumped []*pkg.SourcePackage, message string) (string, error)
}

type sourceNvBumper struct{}

func (sourceNvBumper) NvBump(src *pacmansource.SourceRepo, name, newVersion string, stderr io.Writer) (*pkg.SourcePackage, error) {
	return source.NvBump(src, name, newVersion, stderr)
}

func (sourceNvBumper) Commit(srcDir string, bumped []*pkg.SourcePackage, message string) (string, error) {
	return source.CommitBump(srcDir, bumped, message)
}

// Cmd moves a package to a new upstream version: pkgver rewrite, pkgrel reset,
// fresh checksums and .SRCINFO. The scheduled update workflow drives it with
// the versions `ayaka ci nvcheck` reports.
func Cmd(runtime *app.Runtime) *cobra.Command { return newCommand(sourceNvBumper{}, runtime) }

func newCommand(svc nvBumper, runtime *app.Runtime) *cobra.Command {
	var message string
	var noCommit bool
	cmd := cobra.Command{
		Use:               "nvbump <srcrepo> <pkgname> <newver>",
		Short:             "Set pkgver to a new upstream version, refresh checksums and commit",
		Args:              cobra.ExactArgs(3),
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

			p, err := svc.NvBump(srcrepo, args[1], args[2], cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			slog.Info("bumped pkgver", "pkgbase", p.Base(), "version", p.Version())
			if noCommit {
				return nil
			}

			if message == "" {
				message = "chore: update " + p.Base() + " to " + args[2]
			}
			hash, err := svc.Commit(srcrepo.Dir, []*pkg.SourcePackage{p}, message)
			if err != nil {
				return err
			}
			slog.Info("committed pkgver bump", "commit", hash, "pkgbase", p.Base())
			return nil
		},
	}
	cmd.Flags().StringVar(&message, "message", "", "Commit message (default: chore: update <pkgbase> to <newver>)")
	cmd.Flags().BoolVar(&noCommit, "no-commit", false, "Edit PKGBUILD/.SRCINFO only; leave committing to the caller")
	return &cmd
}
