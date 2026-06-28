package trustcmd

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/kayo/cmd/shared"
	"github.com/Hayao0819/Kamisato/kayo/trust"
	"github.com/spf13/cobra"
)

func trustListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List trusted maintainers and approved packages",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := shared.LoadConfig(cmd)
			if err != nil {
				return err
			}
			store, err := trust.Open(cfg.ResolvedTrustStore())
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "maintainers:")
			for _, m := range store.Maintainers() {
				fmt.Fprintf(out, "  %s/%s\n", m.Source, m.Account)
			}
			fmt.Fprintln(out, "packages:")
			for _, a := range store.Approvals() {
				fmt.Fprintf(out, "  %s @ %s (maintainer %q, source %s)\n", a.Pkgbase, shared.Short(a.Commit), a.Maintainer, a.Source)
			}
			return nil
		},
	}
}
