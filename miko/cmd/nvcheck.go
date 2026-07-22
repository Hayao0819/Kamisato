package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/service"
)

type nvcheckRow struct {
	Pkgbase   string `json:"pkgbase"`
	Published string `json:"published"`
	Upstream  string `json:"upstream"`
	Status    string `json:"status"`
}

const nvcheckDefaultFmt = "table {{.Pkgbase}}\t{{.Published}}\t{{.Upstream}}\t{{.Status}}"

func nvcheckCmd() *cobra.Command {
	cmd := &cobra.Command{
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
			serviceOptions, err := serviceDependencies(cfg)
			if err != nil {
				return err
			}

			s := service.New(cfg, serviceOptions...)
			results := s.CheckUpstreamVersionsDryRun(cmd.Context())

			rows := make([]nvcheckRow, 0, len(results))
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
				rows = append(rows, nvcheckRow{
					Pkgbase:   r.Pkgbase,
					Published: dashIfEmpty(r.Current),
					Upstream:  dashIfEmpty(r.Latest),
					Status:    status,
				})
			}
			format, err := cliutil.ResolveFormat(cmd, nvcheckDefaultFmt)
			if err != nil {
				return err
			}
			header := nvcheckRow{Pkgbase: "PKGBASE", Published: "PUBLISHED", Upstream: "UPSTREAM", Status: "STATUS"}
			if err := cliutil.RenderList(cmd.OutOrStdout(), format, header, rows); err != nil {
				return err
			}

			if outdated > 0 {
				return fmt.Errorf("%d package(s) out of date", outdated)
			}
			return nil
		},
	}
	cliutil.AddFormatFlags(cmd)
	return cmd
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
