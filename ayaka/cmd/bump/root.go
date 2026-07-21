package bumpcmd

import (
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/samber/lo"
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/build"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/gitcmd"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
)

// Cmd raises pkgrel for packages that must rebuild without a source change of
// their own (a dependency update); source edits stay on the ayaka side so ayato
// never touches sources.
func Cmd() *cobra.Command {
	var by string
	var message string
	var noCommit bool
	cmd := cobra.Command{
		Use:   "bump <srcrepo> <pkgname>...",
		Short: "Raise pkgrel for a rebuild and commit the change",
		Args:  cobra.MinimumNArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			app := shared.AppFrom(cmd)
			if len(args) == 0 {
				return app.GetSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
			}
			sr := app.GetSrcRepo(args[0])
			if sr == nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			var cands []string
			for _, p := range sr.Pkgs {
				cands = append(cands, p.Base())
			}
			return cands, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			srcrepo := shared.AppFrom(cmd).GetSrcRepo(args[0])
			if srcrepo == nil {
				return errors.WrapErr(shared.ErrSourceRepoNotFound, args[0])
			}

			bumped, err := build.BumpPkgrel(srcrepo, args[1:], by, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			for _, p := range bumped {
				slog.Info("bumped pkgrel", "pkgbase", p.Base(), "version", p.Version())
			}
			bases := lo.Map(bumped, func(p *pkg.SourcePackage, _ int) string { return p.Base() })
			if noCommit {
				return nil
			}

			root, err := shared.GitRootDir(srcrepo.Dir)
			if err != nil {
				return err
			}
			var paths []string
			for _, p := range bumped {
				for _, f := range []string{"PKGBUILD", ".SRCINFO"} {
					rel, err := filepath.Rel(root, filepath.Join(p.Dir(), f))
					if err != nil {
						return errors.WrapErr(err, "failed to resolve path for commit")
					}
					paths = append(paths, filepath.ToSlash(rel))
				}
			}
			if message == "" {
				message = "chore: rebuild " + strings.Join(bases, " ") + " for dependency update"
			}
			hash, err := gitcmd.CommitPaths(root, paths, message)
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
