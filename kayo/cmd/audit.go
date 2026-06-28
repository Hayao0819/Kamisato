package cmd

import (
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/kayo/audit"
	"github.com/Hayao0819/Kamisato/kayo/trust"
	"github.com/spf13/cobra"
)

func loadConfig(cmd *cobra.Command) (*conf.KayoConfig, error) {
	configFile, _ := cmd.Flags().GetString("config")
	return conf.LoadKayoConfig(cmd.Flags(), configFile)
}

func auditCmd() *cobra.Command {
	var ref string
	var llm bool
	cmd := &cobra.Command{
		Use:   "audit <package|dir|git-url>",
		Short: "Statically audit a PKGBUILD and check maintainer trust",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}

			r, cleanup, err := resolve(cmd.Context(), cfg, args[0], ref)
			defer cleanup()
			if err != nil {
				return err
			}

			report, err := audit.Scan(r.Dir)
			if err != nil {
				return err
			}
			store, err := trust.Open(cfg.ResolvedTrustStore())
			if err != nil {
				return err
			}
			verdict := store.Evaluate(r.Source, r.Pkgbase, r.Maintainer)

			out := cmd.OutOrStdout()
			printReport(out, r, report, verdict)
			printLLMAdvisory(cmd.Context(), out, cfg, r.Dir, llm)
			if report.Max() >= audit.SevHigh {
				return utils.NewErr("audit found high-severity issues")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&ref, "ref", "", "git ref or commit to check out")
	// Not "--llm": that flag name collides with the [llm] config section and the
	// loader would try to decode a bool onto the struct.
	cmd.Flags().BoolVar(&llm, "llm-advisory", false, "also run the LLM advisory pass (overrides config)")
	return cmd
}
