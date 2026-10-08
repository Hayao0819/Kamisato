package addcmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/kayo/cmd/internal/advisory"
	"github.com/Hayao0819/Kamisato/kayo/cmd/internal/output"
	"github.com/Hayao0819/Kamisato/kayo/cmd/internal/settings"
	"github.com/Hayao0819/Kamisato/kayo/service"
)

// Cmd audits and pins a package and vouches for its maintainer ACCOUNT from
// source metadata, never the forgeable git author email.
func Cmd() *cobra.Command {
	var ref string
	var force bool
	cmd := &cobra.Command{
		Use:   "add <package|dir|git-url>",
		Short: "Review and pin a package; vouch for its maintainer account",
		Long: "Audit a package, approve its reviewed commit, and vouch for its maintainer account.\n\n" +
			"Vouching auto-allows a future HANDOFF of an already-approved package to that\n" +
			"account; it does not auto-trust brand-new packages, which still need review.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := settings.Load(cmd.Flags())
			if err != nil {
				return err
			}

			result, err := service.Review(cmd.Context(), cfg, args[0], service.ReviewOptions{Ref: ref, Approve: true, Force: force}, advisory.Checker(cfg.LLM, false))
			if result != nil {
				out := cmd.OutOrStdout()
				output.PrintReport(out, result.Resolved, result.Report, result.Verdict)
				output.PrintLLMAdvisory(out, result.Advisory, result.AdvisoryErr)
				if result.Approved {
					r := result.Resolved
					fmt.Fprintf(out, "trusted and pinned %s at %s (maintainer %q)\n", r.Pkgbase, output.Short(r.Commit), r.Maintainer)
				}
			}
			return err
		},
	}
	cmd.Flags().StringVar(&ref, "ref", "", "git ref or commit to pin")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "trust despite high-severity audit findings")
	return cmd
}
