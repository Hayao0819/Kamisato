package hookcmd

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/ayaka/service/hook"
	"github.com/Hayao0819/Kamisato/internal/errors"
	pacmanhook "github.com/Hayao0819/Kamisato/pkg/pacman/hook"
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
			names := args
			if len(names) == 0 {
				names = pacmanhook.StdinTargets()
			}
			if len(names) == 0 {
				return nil
			}

			files, err := hook.CollectFiles(names, all, cacheOverride, buildDirs, pacmanConf)
			if err != nil {
				return err
			}
			if len(files) == 0 {
				return nil
			}

			api, err := shared.RepoClient(cmd)
			if err != nil {
				// The hook runs as root, so this resolves against root's server db.
				return errors.WrapErr(err, "resolving the ayato server/credentials (set up root's db with 'sudo ayaka server login')")
			}
			// pacman blocks until a PostTransaction hook exits, so a stalled server
			// would hang the whole transaction. Bound the upload via context.
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			err = api.UploadPackageFiles(ctx, repo, files...)
			if err != nil {
				return errors.WrapErr(err, "failed to upload packages (the server may be slow or unreachable)")
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
	_ = cmd.MarkFlagRequired("repo")
	return cmd
}
