package bumpcmd

import (
	"context"
	"io"
	"log/slog"
	"strings"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/completion"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/sourcerepos"
	"github.com/Hayao0819/Kamisato/ayaka/service/source"
	sourcerepo "github.com/Hayao0819/Kamisato/ayaka/source"
	"github.com/Hayao0819/Kamisato/internal/pacman/pkg"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

// bumpFunc is the source-edit operation this command drives.
type bumpFunc func(ctx context.Context, src *sourcerepo.SourceRepo, names []string, by string, stderr io.Writer) ([]*pkg.SourcePackage, error)
type commitFunc func(srcDir string, bumped []*pkg.SourcePackage, message string) (string, error)

// Cmd raises pkgrel for packages that must rebuild without a source change of
// their own (a dependency update); source edits stay on the ayaka side so ayato
// never touches sources.
func Cmd() *cobra.Command {
	return newCommand(source.BumpPkgrel, source.CommitBump, nil, nil)
}

func newCommand(bump bumpFunc, commit commitFunc, resolve sourcerepos.LookupFunc, complete cobra.CompletionFunc) *cobra.Command {
	var options Options
	var cmd cobra.Command
	if resolve == nil {
		sources := sourcerepos.ForCommand(&cmd)
		resolve = sources.Find
		if complete == nil {
			complete = completion.CompleteSrcRepoThenPackages(sources)
		}
	}
	cmd = cobra.Command{
		Use:               "bump <srcrepo> <pkgname>...",
		Short:             "Raise pkgrel for a rebuild and commit the change",
		Args:              cobra.MinimumNArgs(2),
		ValidArgsFunction: complete,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.Repository = args[0]
			options.Packages = args[1:]
			return run(cmd.Context(), cmd.ErrOrStderr(), resolve, bump, commit, options)
		},
	}
	cmd.Flags().StringVar(&options.By, "by", "0.1", "Pkgrel step: 0.1 (1 -> 1.1) or 1 (next integer)")
	cmd.Flags().StringVar(&options.Message, "message", "", "Commit message (default: a rebuild message naming the packages)")
	cmd.Flags().BoolVar(&options.NoCommit, "no-commit", false, "Edit PKGBUILD/.SRCINFO only; leave committing to the caller")
	return &cmd
}

// Options is the parsed input to this command, independent of Cobra.
type Options struct {
	Repository string
	Packages   []string
	By         string
	Message    string
	NoCommit   bool
}

func run(ctx context.Context, stderr io.Writer, resolve sourcerepos.LookupFunc, bump bumpFunc, commit commitFunc, options Options) error {
	srcrepo, err := sourcerepos.Require(resolve, options.Repository)
	if err != nil {
		return err
	}

	bumped, err := bump(ctx, srcrepo, options.Packages, options.By, stderr)
	if err != nil {
		return err
	}
	for _, p := range bumped {
		slog.Info("bumped pkgrel", "pkgbase", p.Base(), "version", p.Version())
	}
	if options.NoCommit {
		return nil
	}

	bases := lo.Map(bumped, func(p *pkg.SourcePackage, _ int) string { return p.Base() })
	if options.Message == "" {
		options.Message = "chore: rebuild " + strings.Join(bases, " ") + " for dependency update"
	}
	hash, err := commit(srcrepo.Dir, bumped, options.Message)
	if err != nil {
		return err
	}
	slog.Info("committed pkgrel bump", "commit", hash, "packages", bases)
	return nil
}
