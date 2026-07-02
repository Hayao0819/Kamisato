package mikocmd

import (
	"slices"

	"github.com/Hayao0819/Kamisato/ayaka/build"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/spf13/cobra"
)

// mikoBuildCmd submits a build to miko: a git/AUR repo (--git), else the local
// PKGBUILD of the named source package. `ayaka build --remote` delegates here.
func mikoBuildCmd() *cobra.Command {
	var (
		gitURL    string
		gitRef    string
		gitSubdir string
		arch      string
		timeout   int
		signLocal bool
		localKey  string
		localPass string
	)
	cmd := &cobra.Command{
		Use:   "build <repo> [packages...]",
		Short: "Submit a build job to miko",
		Args:  cobra.MinimumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return shared.AppFrom(cmd).GetSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// With a git source the repo arg names the destination repo on
			// ayato, which need not exist locally. Otherwise it must be a known
			// source repo.
			if gitURL == "" && !slices.Contains(shared.AppFrom(cmd).GetSrcRepoNames(), args[0]) {
				return utils.WrapErr(shared.ErrInvalidRepoName, args[0])
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
				Timeout:   timeout,
				Pkgs:      args[1:],
			}
			if signLocal {
				if localKey == "" {
					return utils.NewErr("--sign-local requires --local-key")
				}
				return build.RunRemoteBuildLocalSign(cmd.Context(), opts, localKey, localPass)
			}
			return build.RunRemoteBuild(cmd.Context(), opts)
		},
	}
	cmd.Flags().BoolVar(&signLocal, "sign-local", false, "Download the build and sign it locally instead of on miko")
	cmd.Flags().StringVar(&localKey, "local-key", "", "Path to the local OpenPGP private key (with --sign-local)")
	cmd.Flags().StringVar(&localPass, "local-pass", "", "Passphrase for the local key (with --sign-local)")
	cmd.Flags().StringVar(&gitURL, "git", "", "Build from a git/AUR repository URL")
	cmd.Flags().StringVar(&gitRef, "ref", "", "Git ref to build (with --git)")
	cmd.Flags().StringVar(&gitSubdir, "subdir", "", "Subdirectory within the git repository (with --git)")
	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Target architecture for the build")
	cmd.Flags().IntVar(&timeout, "timeout", 0, "Build timeout in minutes (0 uses the server default)")
	return cmd
}
