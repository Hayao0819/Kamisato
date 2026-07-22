package mikocmd

import (
	"fmt"
	"math"
	"time"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/ayaka/service/build"
	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/errors"
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
		Use:               "build <srcrepo> [pkgname...]",
		Short:             "Submit a build job to miko",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: shared.CompleteSrcRepoThenPackages,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if gitURL == "" && app.From(cmd).GetSrcRepo(args[0]) == nil {
				return errors.WrapErr(shared.ErrSourceRepoNotFound, args[0])
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			server, err := cmd.Flags().GetString("server")
			if err != nil {
				return err
			}
			srv, err := shared.ResolveAyatoServer(server)
			if err != nil {
				return err
			}
			api, err := shared.AyatoClient(srv)
			if err != nil {
				return err
			}
			srcrepo := app.From(cmd).GetSrcRepo(args[0])

			opts := build.RemoteBuildOpts{
				Repo:      args[0],
				GitURL:    gitURL,
				GitRef:    gitRef,
				GitSubdir: gitSubdir,
				Arch:      arch,
				Timeout:   durationToMinutes(timeout),
				Pkgs:      args[1:],
			}
			if signLocal {
				passphrase, err := cliutil.ResolveSecret(shared.PassphraseEnv, passphraseFile, nil)
				if err != nil {
					return err
				}
				return build.RunRemoteBuildLocalSign(cmd.Context(), api, srcrepo, opts, localKey, passphrase)
			}
			jobID, err := build.RunRemoteBuild(cmd.Context(), api, srcrepo, opts)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), jobID)
			return nil
		},
	}
	cmd.Flags().BoolVar(&signLocal, "sign-local", false, "Download the build and sign it locally instead of on miko")
	cmd.Flags().StringVar(&localKey, "key", "", "Path to the local OpenPGP private key (with --sign-local)")
	cmd.Flags().StringVar(&passphraseFile, "passphrase-file", "", "File containing the key passphrase; env "+shared.PassphraseEnv+" takes precedence")
	cmd.Flags().StringVar(&gitURL, "git", "", "Build from a git/AUR repository URL")
	cmd.Flags().StringVar(&gitRef, "ref", "", "Git ref to build (with --git)")
	cmd.Flags().StringVar(&gitSubdir, "subdir", "", "Subdirectory within the git repository (with --git)")
	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Target architecture for the build")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "Build timeout (e.g. 30m); 0 uses the server default")
	cmd.MarkFlagsRequiredTogether("sign-local", "key")
	return cmd
}
