package mikocmd

import (
	"math"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/ayaka/build"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/spf13/cobra"
)

// durationToMinutes converts a duration to whole minutes, rounding up.
// Zero (or negative) maps to zero so the server applies its own default.
func durationToMinutes(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	return int(math.Ceil(d.Minutes()))
}

// mikoBuildCmd submits a build to miko: a git/AUR repo (--git), else the local
// PKGBUILD of the named source package.
func mikoBuildCmd() *cobra.Command {
	var (
		gitURL         string
		gitRef         string
		gitSubdir      string
		arch           string
		timeout        time.Duration
		signLocal      bool
		localKey       string
		passphraseFile string
	)
	cmd := &cobra.Command{
		Use:   "build <srcrepo> [pkgname...]",
		Short: "Submit a build job to miko",
		Args:  cobra.MinimumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return shared.AppFrom(cmd).GetSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if gitURL == "" && !slices.Contains(shared.AppFrom(cmd).GetSrcRepoNames(), args[0]) {
				return errwrap.WrapErr(shared.ErrInvalidRepoName, args[0])
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			server, err := cmd.Flags().GetString("server")
			if err != nil {
				return err
			}
			opts := build.RemoteBuildOpts{
				Repo:      args[0],
				Server:    server,
				GitURL:    gitURL,
				GitRef:    gitRef,
				GitSubdir: gitSubdir,
				Arch:      arch,
				Timeout:   durationToMinutes(timeout),
				Pkgs:      args[1:],
			}
			if signLocal {
				passphrase := os.Getenv("AYAKA_SIGN_PASSPHRASE")
				if passphrase == "" && passphraseFile != "" {
					data, err := os.ReadFile(passphraseFile)
					if err != nil {
						return errwrap.WrapErr(err, "failed to read passphrase file")
					}
					passphrase = strings.TrimRight(string(data), "\n")
				}
				return build.RunRemoteBuildLocalSign(cmd.Context(), opts, localKey, passphrase)
			}
			return build.RunRemoteBuild(cmd.Context(), opts)
		},
	}
	cmd.Flags().BoolVar(&signLocal, "sign-local", false, "Download the build and sign it locally instead of on miko")
	cmd.Flags().StringVar(&localKey, "key", "", "Path to the local OpenPGP private key (with --sign-local)")
	cmd.Flags().StringVar(&passphraseFile, "passphrase-file", "", "File containing the key passphrase; env AYAKA_SIGN_PASSPHRASE takes precedence")
	cmd.Flags().StringVar(&gitURL, "git", "", "Build from a git/AUR repository URL")
	cmd.Flags().StringVar(&gitRef, "ref", "", "Git ref to build (with --git)")
	cmd.Flags().StringVar(&gitSubdir, "subdir", "", "Subdirectory within the git repository (with --git)")
	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Target architecture for the build")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "Build timeout (e.g. 30m); 0 uses the server default")
	cmd.MarkFlagsRequiredTogether("sign-local", "key")
	return cmd
}
