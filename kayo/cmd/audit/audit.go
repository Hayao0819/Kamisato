package auditcmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/kayo/cmd/internal/advisory"
	"github.com/Hayao0819/Kamisato/kayo/cmd/internal/output"
	"github.com/Hayao0819/Kamisato/kayo/cmd/internal/settings"
	"github.com/Hayao0819/Kamisato/kayo/service"
)

func Cmd() *cobra.Command {
	var options service.ReviewOptions
	var forceAdvisory bool
	cmd := &cobra.Command{
		Use:   "audit <package|dir|git-url>",
		Short: "Statically audit a PKGBUILD and check maintainer trust",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := settings.Load(cmd.Flags())
			if err != nil {
				return err
			}
			result, err := service.Review(cmd.Context(), cfg, args[0], options, advisory.Checker(cfg.LLM, forceAdvisory))
			if result != nil {
				output.PrintReport(cmd.OutOrStdout(), result.Resolved, result.Report, result.Verdict)
				output.PrintLLMAdvisory(cmd.OutOrStdout(), result.Advisory, result.AdvisoryErr)
			}
			return err
		},
	}
	cmd.Flags().StringVar(&options.Ref, "ref", "", "git ref or commit to check out")
	// Not "--llm": that flag name collides with the [llm] config section.
	cmd.Flags().BoolVar(&forceAdvisory, "llm-advisory", false, "also run the LLM advisory pass (overrides config)")
	return cmd
}
