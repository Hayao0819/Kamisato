package hookcmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
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
			// client has no timeout, so a stalled server would hang the whole
			// transaction. Bound the upload and fail the hook instead.
			done := make(chan error, 1)
			go func() { done <- client.UploadPackageFiles(repo, files...) }()
			select {
			case err := <-done:
				if err != nil {
					return utils.WrapErr(err, "failed to upload packages")
				}
			case <-time.After(timeout):
				return utils.NewErrf("upload timed out after %s; the ayato server may be slow or unreachable", timeout)
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

// makepkgPkgDestScript sources makepkg's config files in makepkg's own order
// (load_makepkg_config) and prints the resolved PKGDEST, so variable expansion
// and includes are interpreted by bash exactly as makepkg would, not guessed.
const makepkgPkgDestScript = `confdir=/etc
[[ -r $confdir/makepkg.conf ]] && source "$confdir/makepkg.conf"
if [[ -d $confdir/makepkg.conf.d ]]; then
  for f in "$confdir/makepkg.conf.d"/*.conf; do
    [[ -r $f ]] && source "$f"
  done
fi
if [[ -r ${XDG_CONFIG_HOME:-$HOME/.config}/pacman/makepkg.conf ]]; then
  source "${XDG_CONFIG_HOME:-$HOME/.config}/pacman/makepkg.conf"
elif [[ -r $HOME/.makepkg.conf ]]; then
  source "$HOME/.makepkg.conf"
fi
printf '%s' "${PKGDEST:-}"`

// makepkgPkgDest returns the PKGDEST makepkg would write a built package to, by
// actually running bash to evaluate makepkg.conf (a pacman-hook system always has
// makepkg and bash). That is where a `-U`-installed foreign package can be found;
// the pacman cache cannot. Empty when PKGDEST is unset (built packages stay in
// the build dir, unknowable to a hook that only gets package names).
func makepkgPkgDest() []string {
	out, err := exec.Command("bash", "-c", makepkgPkgDestScript).Output()
	if err != nil {
		return nil
	}
	if dest := strings.TrimSpace(string(out)); dest != "" {
		return []string{dest}
	}
	return nil
}

// foreignPackages returns the set of installed packages no sync repo provides
// (AUR or locally built). pacman -Qmq exits 1 with empty stdout AND empty stderr
// when none are installed (a normal state); a genuine failure writes to stderr,
// so only the no-match signature is treated as an empty set.
func foreignPackages() (map[string]bool, error) {
	out, err := exec.Command("pacman", "-Qmq").Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 1 && len(out) == 0 && len(ee.Stderr) == 0 {
			return map[string]bool{}, nil
		}
		return nil, utils.WrapErr(err, "pacman -Qmq")
	}
	set := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			set[line] = true
		}
	}
	return set, nil
}

func filterForeign(names []string, foreign map[string]bool) []string {
	var out []string
	for _, n := range names {
		if foreign[n] {
			out = append(out, n)
		}
	}
	return out
}

// installedVersion returns the version pacman records for an installed package,
// which (with the name) pins the exact built file in the cache.
func installedVersion(name string) (string, error) {
	out, err := exec.Command("pacman", "-Q", name).Output()
	if err != nil {
		return "", utils.WrapErr(err, "pacman -Q "+name)
	}
	fields := strings.Fields(string(out))
	if len(fields) < 2 {
		return "", utils.NewErrf("unexpected 'pacman -Q' output for %s", name)
	}
	return fields[1], nil
}

// pkgFileTail matches what must follow the name-version- prefix of a built
// package file: a single arch field (no dash), .pkg.tar, and at most one
// compression suffix. The arch field having no dash stops a different package
// whose name+version concatenation aligns (e.g. foo-1.0-1- matching a foo-1.0
// build) from matching, and the end anchor rejects .sig and .part sidecars.
var pkgFileTail = regexp.MustCompile(`^[^-]+\.pkg\.tar(\.[A-Za-z0-9]+)?$`)

// findCachedPackage finds the built package file for name-version in the cache
// dirs. The name-version- prefix plus the strict tail pins exactly the file for
// this package, excluding signatures, partial downloads, and look-alikes.
func findCachedPackage(dirs []string, name, version string) (string, bool) {
	prefix := name + "-" + version + "-"
	for _, d := range dirs {
		matches, _ := filepath.Glob(filepath.Join(d, prefix+"*"))
		for _, m := range matches {
			if pkgFileTail.MatchString(strings.TrimPrefix(filepath.Base(m), prefix)) {
				return m, true
			}
		}
	}
	return "", false
}
