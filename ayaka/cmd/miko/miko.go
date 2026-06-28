package mikocmd

import (
	"slices"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/spf13/cobra"
)

// Cmd groups the client commands for the miko build service. ayaka never
// talks to miko directly: every request goes to an ayato endpoint, which
// reverse-proxies it to miko. The --server flag therefore names an ayato
// server, and is shared by all miko subcommands.
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "miko",
		Short: "Submit and inspect builds on the miko build service",
		Long:  "Submit build jobs to miko (via ayato) and inspect their status and logs.",
	}
	cmd.PersistentFlags().StringP("server", "s", "", "ayato server that relays to miko (default: serverdb default)")
	cmd.AddCommand(
		mikoBuildCmd(),
		mikoJobsCmd(),
		mikoStatusCmd(),
		mikoLogsCmd(),
		mikoCancelCmd(),
		mikoStatsCmd(),
	)
	return cmd
}

// mikoBuildCmd submits a build job to miko through ayato and prints the job id.
// The source is either a git/AUR repository (--git) or, by default, the local
// PKGBUILD of the named source package. `ayaka build --remote` delegates here.
func mikoBuildCmd() *cobra.Command {
	var (
		gpgkey    string
		gitURL    string
		gitRef    string
		gitSubdir string
		arch      string
		timeout   int
	)
	cmd := &cobra.Command{
		Use:   "build <repo> [packages...]",
		Short: "Submit a build job to miko",
		Args:  cobra.MinimumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return shared.GetSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// With a git source the repo arg names the destination repo on
			// ayato, which need not exist locally. Otherwise it must be a known
			// source repo.
			if gitURL == "" && !slices.Contains(shared.GetSrcRepoNames(), args[0]) {
				return utils.WrapErr(shared.ErrInvalidRepoName, args[0])
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			server, err := cmd.Flags().GetString("server")
			if err != nil {
				return err
			}
			return shared.RunRemoteBuild(shared.RemoteBuildOpts{
				Repo:      args[0],
				Server:    server,
				GPGKey:    gpgkey,
				GitURL:    gitURL,
				GitRef:    gitRef,
				GitSubdir: gitSubdir,
				Arch:      arch,
				Timeout:   timeout,
				Pkgs:      args[1:],
			})
		},
	}
	cmd.Flags().StringVarP(&gpgkey, "key", "g", "", "GPG key id for miko to sign with")
	cmd.Flags().StringVar(&gitURL, "git", "", "Build from a git/AUR repository URL")
	cmd.Flags().StringVar(&gitRef, "ref", "", "Git ref to build (with --git)")
	cmd.Flags().StringVar(&gitSubdir, "subdir", "", "Subdirectory within the git repository (with --git)")
	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Target architecture for the build")
	cmd.Flags().IntVar(&timeout, "timeout", 0, "Build timeout in minutes (0 uses the server default)")
	return cmd
}
