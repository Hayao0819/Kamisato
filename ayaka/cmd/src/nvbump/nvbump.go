package nvbumpcmd

import (
	"context"
	"io"
	"log/slog"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/completion"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/sourcerepos"
	"github.com/Hayao0819/Kamisato/ayaka/service/source"
	sourcerepo "github.com/Hayao0819/Kamisato/ayaka/source"
	"github.com/Hayao0819/Kamisato/internal/pacman/pkg"
	"github.com/spf13/cobra"
)

// bumpFunc is the source-edit operation this command drives.
type bumpFunc func(ctx context.Context, src *sourcerepo.SourceRepo, name, newVersion string, stderr io.Writer) (*pkg.SourcePackage, error)
type commitFunc func(srcDir string, bumped []*pkg.SourcePackage, message string) (string, error)

// Cmd moves a package to a new upstream version: pkgver rewrite, pkgrel reset,
// fresh checksums and .SRCINFO. The scheduled update workflow drives it with
// the versions `ayaka ci nvcheck` reports.
func Cmd() *cobra.Command {
	return newCommand(source.NvBump, source.CommitBump, nil, nil)
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
		Use:               "nvbump <srcrepo> <pkgname> <newver>",
		Short:             "Set pkgver to a new upstream version, refresh checksums and commit",
		Args:              cobra.ExactArgs(3),
		ValidArgsFunction: complete,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.Repository = args[0]
			options.Package = args[1]
			options.Version = args[2]
			return run(cmd.Context(), cmd.ErrOrStderr(), resolve, bump, commit, options)
		},
	}
	cmd.Flags().StringVar(&options.Message, "message", "", "Commit message (default: chore: update <pkgbase> to <newver>)")
	cmd.Flags().BoolVar(&options.NoCommit, "no-commit", false, "Edit PKGBUILD/.SRCINFO only; leave committing to the caller")
	return &cmd
}

// Options is the parsed input to this command, independent of Cobra.
type Options struct {
	Repository string
	Package    string
	Version    string
	Message    string
	NoCommit   bool
}

func run(ctx context.Context, stderr io.Writer, resolve sourcerepos.LookupFunc, bump bumpFunc, commit commitFunc, options Options) error {
	srcrepo, err := sourcerepos.Require(resolve, options.Repository)
	if err != nil {
		return err
	}

	p, err := bump(ctx, srcrepo, options.Package, options.Version, stderr)
	if err != nil {
		return err
	}
	slog.Info("bumped pkgver", "pkgbase", p.Base(), "version", p.Version())
	if options.NoCommit {
		return nil
	}

	if options.Message == "" {
		options.Message = "chore: update " + p.Base() + " to " + options.Version
	}
	hash, err := commit(srcrepo.Dir, []*pkg.SourcePackage{p}, options.Message)
	if err != nil {
		return err
	}
	slog.Info("committed pkgver bump", "commit", hash, "pkgbase", p.Base())
	return nil
}
