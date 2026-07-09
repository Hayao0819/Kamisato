package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/service"
)

func nvcheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "nvcheck",
		Short: "Check monitored packages for newer upstream versions",
		Long: "Fetch the latest upstream version of every entry under nvcheck.entries " +
			"and compare it against the version published on ayato. This is a read-only " +
			"report: it does not enqueue rebuilds (the running server does that on its " +
			"nvcheck.interval_min ticker). Exits non-zero when any entry is out of date.",
		RunE: func(cmd *cobra.Command, args []string) error {
			configFile, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			cfg, err := conf.LoadMikoConfig(cmd.Flags(), configFile)
			if err != nil {
				return err
			}

			s, ok := service.New(cfg, nil, nil, nil).(*service.Service)
			if !ok {
				return fmt.Errorf("unexpected service type")
			}
			results := s.CheckUpstreamVersionsDryRun(cmd.Context())

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "PKGBASE\tPUBLISHED\tUPSTREAM\tSTATUS")
			outdated := 0
			for _, r := range results {
				status := "up-to-date"
				switch {
				case r.Err != nil:
					status = "error: " + r.Err.Error()
				case r.Outdated:
					status = "OUTDATED"
					outdated++
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Pkgbase, dashIfEmpty(r.Current), dashIfEmpty(r.Latest), status)
			}
			_ = w.Flush()

			if outdated > 0 {
				return fmt.Errorf("%d package(s) out of date", outdated)
			}
			return nil
		},
	}
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
