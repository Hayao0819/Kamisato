package repocmd

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/build"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/errors"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

func repoRemoveCmd() *cobra.Command {
	var diffMode bool
	var arch string
	var dryRun bool
	var diffURL string
	cmd := &cobra.Command{
		Use:   "remove <repo> <pkgname>... | --diff <srcrepo>",
		Short: "Remove packages by name, or (--diff) prune ones no longer in a source repo",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if diffMode {
				return runPrune(cmd, args[0], arch, diffURL, dryRun)
			}
			if len(args) < 2 {
				return errors.NewErr("remove needs <repo> <pkgname>... (or --diff <srcrepo>)")
			}
			client, err := shared.RepoClient(cmd)
			if err != nil {
				return err
			}
			for _, name := range args[1:] {
				if err := client.RemovePackageAllArchitectures(cmd.Context(), args[0], name); err != nil {
					return err
				}
			}
			return nil
		},
	}
	shared.AddRepoServerFlags(cmd)
	cmd.Flags().BoolVar(&diffMode, "diff", false, "Prune packages no longer present in the source repo (arg is the source repo)")
	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Architecture whose remote db defines the current package set (with --diff)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "With --diff, list what would be removed without deleting")
	cmd.Flags().StringVar(&diffURL, "diff-url", "", "Remote repo db dir for --diff (.../repo/<repo>/<arch>); overrides repo.json url")
	return cmd
}

// runPrune removes the packages an ayato repo still has that its source repo no
// longer provides (a deleted PKGBUILD). The source repo names the desired set and
// its public db names the present set; the difference is deleted.
func runPrune(cmd *cobra.Command, srcName, arch, diffURL string, dryRun bool) error {
	src := shared.AppFrom(cmd).GetSrcRepo(srcName)
	if src == nil {
		return errors.WrapErr(shared.ErrSourceRepoNotFound, srcName)
	}

	var desired []string
	for _, sp := range src.Pkgs {
		desired = append(desired, sp.Names()...)
	}
	// Refuse to prune from an empty desired set: that is almost always a source read
	// error, and pruning would delete the whole repo.
	if len(desired) == 0 {
		return errors.NewErr("source repo " + srcName + " has no packages; refusing to prune")
	}

	// The db dir is --diff-url (already arch-specific) when given, else repo.json's url
	// with the arch appended.
	dburl := diffURL
	if dburl == "" {
		if src.Config.URL == "" {
			return errors.NewErr("source repo " + srcName + " has no url in repo.json; pass --diff-url")
		}
		dburl = strings.TrimRight(src.Config.URL, "/") + "/" + arch
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
