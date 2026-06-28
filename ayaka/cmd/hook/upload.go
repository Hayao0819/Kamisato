package hookcmd

import (
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/pacman/alpm"
	"github.com/Hayao0819/Kamisato/pkg/pacman/hook"
	"github.com/spf13/cobra"
)

// hookUploadCmd is the hook's pacman entry point. pacman does not copy
// `pacman -U`-installed files (locally-built foreign packages) into CacheDir, so
// set makepkg's PKGDEST or --build-dir; those dirs are searched before the cache.
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
				names = hook.StdinTargets()
			}
			if len(names) == 0 {
				return nil
			}

			// By default only publish foreign (AUR/local) packages; the hook's
			// Target=* otherwise fires for every official-repo package, which
			// already lives on mirrors.
			if !all {
				foreign, err := alpm.ForeignPackages()
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
				dirs = append(append(append([]string{}, buildDirs...), makepkgPkgDest()...), alpm.CacheDirs(pacmanConf)...)
			}

			var files []string
			for _, name := range names {
				ver, err := alpm.InstalledVersion(name)
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
			// pacman blocks until a PostTransaction hook exits and blinky uses
			// http.DefaultClient (no timeout), so a stalled server would hang the
			// whole transaction. Bound it; mutating the global client is safe in
			// this one-shot.
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
