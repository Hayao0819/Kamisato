package hookcmd

import (
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/pacmanhook"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/spf13/cobra"
)

// hookUploadCmd is the hook's runtime entry point. pacman feeds it the installed
// package names; it locates each one's built file and uploads it to the repo.
//
// IMPORTANT: pacman does NOT copy `pacman -U`-installed files (how makepkg/paru/
// yay install a locally-built package) into CacheDir, so the cache alone cannot
// locate foreign packages — exactly the ones this hook exists to publish. Set
// makepkg's PKGDEST (or pass --build-dir) to a persistent directory so built
// packages land somewhere the hook can find them; that directory is searched
// before the cache. A package whose file is found nowhere is logged and skipped.
func hookUploadCmd() *cobra.Command {
	var repo, pacmanConf string
	var cacheOverride, buildDirs []string
	var all bool
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "upload [pkgname...]",
		Short: "Upload freshly installed packages to the repo (pacman hook entry point)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if repo == "" {
				return utils.NewErr("--repo is required")
			}
			names := args
			if len(names) == 0 {
				names = pacmanhook.StdinTargets()
			}
			if len(names) == 0 {
				return nil
			}

			// By default only publish foreign packages (AUR/locally built); the
			// hook's Target=* otherwise fires for every official-repo package in a
			// -Syu, which already lives on mirrors and shouldn't flood the repo.
			if !all {
				foreign, err := foreignPackages()
				if err != nil {
					return utils.WrapErr(err, "could not determine foreign packages; pass --all to upload every target")
				}
				names = filterForeign(names, foreign)
				if len(names) == 0 {
					slog.Info("no foreign (AUR/local) packages to upload in this transaction")
					return nil
				}
			}

			// Search build-output dirs (PKGDEST / --build-dir) before the cache:
			// foreign packages live in the former, repo downloads in the latter.
			dirs := cacheOverride
			if len(dirs) == 0 {
				dirs = append(append(append([]string{}, buildDirs...), makepkgPkgDest()...), pacmanhook.CacheDirs(pacmanConf)...)
			}

			var files []string
			for _, name := range names {
				ver, err := installedVersion(name)
				if err != nil {
					slog.Warn("skipping package not in the local db", "name", name, "error", err)
					continue
				}
				path, ok := findCachedPackage(dirs, name, ver)
				if !ok {
					slog.Warn("no package file found; skipping upload (set makepkg PKGDEST or --build-dir for locally-built packages)", "name", name, "version", ver)
					continue
				}
				files = append(files, path)
			}
			if len(files) == 0 {
				return nil
			}

			client, err := shared.RepoClient(cmd)
			if err != nil {
				// The hook runs as root, so this resolves against root's server db.
				return utils.WrapErr(err, "resolving the ayato server/credentials (set up root's db with 'sudo ayaka server login')")
			}
			// pacman blocks until a PostTransaction hook exits, and the blinky
			// client uses http.DefaultClient with no timeout, so a stalled or
			// unreachable server would hang the whole transaction. Bound the
			// request so that on the deadline the client aborts the in-flight
			// upload (it is cancelled, not left running in the background) and
			// returns an error. Mutating the process-global client is safe here:
			// this hook is a one-shot invocation whose only network call is this
			// upload.
			http.DefaultClient.Timeout = timeout
			if err := client.UploadPackageFiles(repo, files...); err != nil {
				return utils.WrapErr(err, "failed to upload packages (the server may be slow or unreachable)")
			}
			out := cmd.OutOrStdout()
			for _, f := range files {
				fmt.Fprintf(out, "uploaded %s\n", filepath.Base(f))
			}
			return nil
		},
	}
	shared.AddRepoServerFlags(cmd)
	cmd.Flags().StringVar(&repo, "repo", "", "target repository on ayato (required)")
	cmd.Flags().StringVar(&pacmanConf, "pacman-config", "", "pacman.conf path for resolving CacheDir (default: pacman's own)")
	cmd.Flags().StringArrayVar(&cacheOverride, "cache-dir", nil, "override the package cache dir(s) instead of reading pacman.conf")
	cmd.Flags().StringArrayVar(&buildDirs, "build-dir", nil, "extra dir(s) holding locally-built packages (e.g. makepkg PKGDEST), searched before the cache")
	cmd.Flags().BoolVar(&all, "all", false, "upload every target, not just foreign (AUR/local) packages")
	cmd.Flags().DurationVar(&timeout, "timeout", 120*time.Second, "max time to wait for the upload before failing the hook")
	return cmd
}
