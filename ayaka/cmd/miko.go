package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"

	"github.com/Hayao0819/Kamisato/internal/ayatoclient"
	"github.com/Hayao0819/Kamisato/internal/utils"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/spf13/cobra"
)

// mikoCmd groups the client commands for the miko build service. ayaka never
// talks to miko directly: every request goes to an ayato endpoint, which
// reverse-proxies it to miko. The --server flag therefore names an ayato
// server, and is shared by all miko subcommands.
func mikoCmd() *cobra.Command {
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
				return getSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// With a git source the repo arg names the destination repo on
			// ayato, which need not exist locally. Otherwise it must be a known
			// source repo.
			if gitURL == "" && !slices.Contains(getSrcRepoNames(), args[0]) {
				return utils.WrapErr(ErrInvalidRepoName, args[0])
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			server, err := cmd.Flags().GetString("server")
			if err != nil {
				return err
			}
			return runRemoteBuild(remoteBuildOpts{
				repo:      args[0],
				server:    server,
				gpgkey:    gpgkey,
				gitURL:    gitURL,
				gitRef:    gitRef,
				gitSubdir: gitSubdir,
				arch:      arch,
				timeout:   timeout,
				pkgs:      args[1:],
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

// remoteBuildOpts collects the inputs for a server-side build submission.
type remoteBuildOpts struct {
	repo      string
	server    string
	gpgkey    string
	gitURL    string
	gitRef    string
	gitSubdir string
	arch      string
	timeout   int
	pkgs      []string
}

// runRemoteBuild submits a build to ayato and prints the resulting job id. The
// source is either a git repository (--git) or, by default, the local PKGBUILD
// of the named source package in the repo.
func runRemoteBuild(o remoteBuildOpts) error {
	srv, err := resolveAyatoServer(o.server)
	if err != nil {
		return err
	}

	arch := o.arch
	if arch == "" {
		arch = "x86_64"
	}

	req := &ayatoclient.BuildRequest{
		Repo:        o.repo,
		Arch:        arch,
		InstallPkgs: o.pkgs,
		GPGKey:      o.gpgkey,
		Timeout:     o.timeout,
	}

	if o.gitURL != "" {
		req.Git = &ayatoclient.GitSource{
			URL:    o.gitURL,
			Ref:    o.gitRef,
			Subdir: o.gitSubdir,
		}
	} else {
		pkgbuild, files, err := readLocalSource(o.repo, o.pkgs)
		if err != nil {
			return err
		}
		req.Pkgbuild = pkgbuild
		req.Files = files
		// install_pkgs targets local package files on the builder, not source
		// package names, so don't pass the selected build packages through.
		req.InstallPkgs = nil
	}

	slog.Info("submitting remote build", "server", srv.URL, "repo", o.repo)
	jobID, err := ayatoclient.SubmitBuild(srv.URL, srv.Username, srv.Password, req)
	if err != nil {
		return utils.WrapErr(err, "failed to submit build")
	}

	fmt.Println(jobID)
	return nil
}

// readLocalSource reads the PKGBUILD and accompanying files of a source package
// in the named repo. When pkgs names a single package that one is used;
// otherwise the repo must hold exactly one source package.
func readLocalSource(repo string, pkgs []string) (string, map[string]string, error) {
	srcrepo := getSrcRepo(repo)
	if srcrepo == nil {
		return "", nil, utils.WrapErr(ErrSourceRepoNotFound, repo)
	}

	srcpkg, err := selectSourcePkg(srcrepo, pkgs)
	if err != nil {
		return "", nil, err
	}

	dir := srcpkg.Dir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil, utils.WrapErr(err, "failed to read source directory")
	}

	var pkgbuild string
	files := map[string]string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "", nil, utils.WrapErr(err, "failed to read "+name)
		}
		if name == "PKGBUILD" {
			pkgbuild = string(content)
			continue
		}
		files[name] = string(content)
	}

	if pkgbuild == "" {
		return "", nil, utils.NewErr("PKGBUILD not found in " + dir)
	}
	return pkgbuild, files, nil
}

// selectSourcePkg picks the source package to submit. With no package named it
// requires the repo to hold exactly one; otherwise it matches a single named
// package by pkgbase or package name.
func selectSourcePkg(srcrepo *pacmanrepo.SourceRepo, pkgs []string) (*pkg.SourcePackage, error) {
	if len(pkgs) == 0 {
		switch len(srcrepo.Pkgs) {
		case 0:
			return nil, utils.NewErr("no source packages found in repository")
		case 1:
			return srcrepo.Pkgs[0], nil
		default:
			return nil, utils.NewErr("repository holds multiple packages; specify one to build remotely")
		}
	}
	if len(pkgs) > 1 {
		return nil, utils.NewErr("remote build accepts only one package at a time")
	}

	name := pkgs[0]
	for _, p := range srcrepo.Pkgs {
		if p.Base() == name || slices.Contains(p.Names(), name) {
			return p, nil
		}
	}
	return nil, utils.NewErr("package not found: " + name)
}

func init() {
	subCmds.Add(mikoCmd())
}
