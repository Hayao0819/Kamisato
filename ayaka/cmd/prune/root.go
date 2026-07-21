package prunecmd

import (
	"fmt"
	"log/slog"

	"github.com/samber/lo"
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/build"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/errors"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// Cmd is the promoted top-level form of `repo remove --diff`: it deletes from
// ayato what the source repo no longer provides.
func Cmd() *cobra.Command {
	var arch string
	var dryRun bool
	var diffURL string
	cmd := cobra.Command{
		Use:   "prune <srcrepo>",
		Short: "Remove packages from ayato that are no longer in the source repo",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return shared.AppFrom(cmd).GetSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(cmd, args[0], arch, diffURL, dryRun)
		},
	}
	shared.AddRepoServerFlags(&cmd)
	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Architecture whose remote db defines the current package set")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "List what would be removed without deleting")
	cmd.Flags().StringVar(&diffURL, "diff-url", "", "Remote repo db dir (.../repo/<repo>/<arch>); overrides repo.json url")
	return &cmd
}

// Run removes the packages an ayato repo still has that its source repo no
// longer provides (a deleted PKGBUILD). The source repo names the desired set and
// its public db names the present set; the difference is deleted.
func Run(cmd *cobra.Command, srcName, arch, diffURL string, dryRun bool) error {
	src := shared.AppFrom(cmd).GetSrcRepo(srcName)
	if src == nil {
		return errors.WrapErr(shared.ErrSourceRepoNotFound, srcName)
	}

	desired := lo.FlatMap(src.Pkgs, func(sp *pkg.SourcePackage, _ int) []string { return sp.Names() })
	// Refuse to prune from an empty desired set: that is almost always a source read
	// error, and pruning would delete the whole repo.
	if len(desired) == 0 {
		return errors.NewErr("source repo " + srcName + " has no packages; refusing to prune")
	}

	dburl := shared.ResolveDiffServer(diffURL, "", src.Config.URL, arch)
	if dburl == "" {
		return errors.NewErr("source repo " + srcName + " has no url in repo.json; pass --diff-url")
	}
	rr, err := pacmanrepo.RepoFromURL(dburl, src.Config.Name)
	if errors.Is(err, pacmanrepo.ErrRepoNotFound) {
		slog.Info("remote repo db not found; nothing to prune", "url", dburl)
		return nil
	} else if err != nil {
		return errors.WrapErr(err, "failed to read remote repo db")
	}

	prune := build.PrunablePackages(desired, rr)
	if len(prune) == 0 {
		slog.Info("nothing to prune", "repo", src.Config.Name, "arch", arch)
		return nil
	}
	// Dry run prints the prunable pkgnames to stdout, one per line, so a caller (e.g.
	// a CI prune step deleting via X-API-Key) can consume the list; the human-facing
	// summary goes to the log (stderr).
	if dryRun {
		slog.Info("prunable packages removed from source", "repo", src.Config.Name, "arch", arch, "count", len(prune))
		for _, p := range prune {
			fmt.Println(p)
		}
		return nil
	}

	client, err := shared.RepoClient(cmd)
	if err != nil {
		return err
	}
	slog.Info("pruning packages removed from source", "repo", src.Config.Name, "arch", arch, "packages", prune)
	for _, name := range prune {
		if err := client.RemovePackageAllArchitectures(cmd.Context(), src.Config.Name, name); err != nil {
			return err
		}
	}
	return nil
}
