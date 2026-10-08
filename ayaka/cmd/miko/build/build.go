package buildcmd

import (
	"fmt"
	"math"
	"time"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/completion"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/keyinput"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/remote"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/sourcerepos"
	"github.com/Hayao0819/Kamisato/ayaka/service/build"
	"github.com/Hayao0819/Kamisato/ayaka/source"
	"github.com/Hayao0819/Kamisato/internal/cli"
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
func newCommand(sources sourcerepos.Reader) *cobra.Command {
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
	cmd := &cobra.Command{}
	if sources == nil {
		sources = sourcerepos.ForCommand(cmd)
	}
	*cmd = cobra.Command{
		Use:               "build <srcrepo> [pkgname...]",
		Short:             "Submit a build job to miko",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completion.CompleteSrcRepoThenPackages(sources),
		RunE: func(cmd *cobra.Command, args []string) error {
			srcrepo, err := resolveBuildSource(sources, args[0], gitURL)
			if err != nil {
				return err
			}
			server, err := cmd.Flags().GetString("server")
			if err != nil {
				return err
			}
			srv, err := remote.DefaultProvider().Resolve(server)
			if err != nil {
				return err
			}
			api, err := remote.DefaultProvider().StoredClient(srv)
			if err != nil {
				return err
			}

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
				passphrase, err := cli.ResolveSecret(keyinput.PassphraseEnv, passphraseFile, nil)
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
	cmd.Flags().StringVar(&passphraseFile, "passphrase-file", "", "File containing the key passphrase; env "+keyinput.PassphraseEnv+" takes precedence")
	cmd.Flags().StringVar(&gitURL, "git", "", "Build from a git/AUR repository URL")
	cmd.Flags().StringVar(&gitRef, "ref", "", "Git ref to build (with --git)")
	cmd.Flags().StringVar(&gitSubdir, "subdir", "", "Subdirectory within the git repository (with --git)")
	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Target architecture for the build")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "Build timeout (e.g. 30m); 0 uses the server default")
	cmd.MarkFlagsRequiredTogether("sign-local", "key")
	return cmd
}

func Cmd() *cobra.Command { return newCommand(nil) }

// A Git build carries its own source; only inline local builds need repo input.
func resolveBuildSource(sources sourcerepos.Reader, name, gitURL string) (*source.SourceRepo, error) {
	if gitURL != "" {
		return nil, nil
	}
	return sourcerepos.Require(sources.Find, name)
}
