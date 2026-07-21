package plancmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/build"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/errors"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// Cmd computes the build set for one run from the source repo and the published
// repo db alone, so it is idempotent and needs no server-side build state.
func Cmd() *cobra.Command {
	var arch string
	var diffURL string
	var cascade string
	var costsFile string
	var format string
	var workers int
	var updateSrcinfo bool
	cmd := cobra.Command{
		Use:   "plan <srcrepo>",
		Short: "Compute which packages to build (version diff + rebuild cascade)",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return shared.AppFrom(cmd).GetSrcRepoNames(), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			server, err := cmd.Flags().GetString("server")
			if err != nil {
				return err
			}
			mode, err := build.ParseCascadeMode(cascade)
			if err != nil {
				return err
			}
			if format != "lines" && format != "json" {
				return errors.NewErr("invalid format: " + format + " (lines or json)")
			}

			srcrepo := shared.AppFrom(cmd).GetSrcRepo(args[0])
			if srcrepo == nil {
				return errors.WrapErr(shared.ErrSourceRepoNotFound, args[0])
			}
			if updateSrcinfo {
				srcrepo, err = shared.ReloadWithSrcinfo(srcrepo, cmd.ErrOrStderr())
				if err != nil {
					return err
				}
			}

			dburl := shared.ResolveDiffServer(diffURL, server, srcrepo.Config.URL, arch)
			if dburl == "" {
				return errors.NewErr("source repo " + args[0] + " has no url in repo.json; pass --diff-url")
			}
			rr, err := pacmanrepo.RepoFromURL(dburl, srcrepo.Config.Name)
			if errors.Is(err, pacmanrepo.ErrRepoNotFound) {
				rr = &pacmanrepo.RemoteRepo{Name: srcrepo.Config.Name}
			} else if err != nil {
				return errors.WrapErr(err, "failed to read remote repo db")
			}

			var costs map[string]float64
			if costsFile != "" {
				data, err := os.ReadFile(costsFile)
				if err != nil {
					return errors.WrapErr(err, "failed to read costs file")
				}
				if err := json.Unmarshal(data, &costs); err != nil {
					return errors.WrapErr(err, "failed to parse costs file")
				}
			}

			plan, err := build.ComputePlan(srcrepo.Pkgs, rr, arch, mode, workers, costs)
			if err != nil {
				return err
			}

			if format == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(plan)
			}
			for _, pb := range plan.Order {
				cmd.Println(pb)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Target architecture to plan for")
	cmd.Flags().StringVar(&diffURL, "diff-url", "", "Remote repo db dir (.../repo/<repo>/<arch>); overrides repo.json url")
	shared.AddServerFlag(&cmd)
	_ = cmd.Flags().MarkDeprecated("server", "use --diff-url to point the plan at the remote repo db dir")
	cmd.Flags().StringVar(&cascade, "cascade", "makedepends", "Rebuild propagation: off, makedepends, soname or both")
	cmd.Flags().IntVar(&workers, "workers", 0, "Split the build set into at most N cost-balanced buckets (0 = flat list)")
	cmd.Flags().StringVar(&costsFile, "costs", "", "JSON file of past build times per pkgbase for bucket balancing")
	cmd.Flags().StringVar(&format, "format", "lines", "Output format: lines (pkgbase per line) or json")
	cmd.Flags().BoolVar(&updateSrcinfo, "update-srcinfo", true, "Regenerate .SRCINFO from PKGBUILD before planning (requires makepkg; skipped when absent)")
	return &cmd
}
