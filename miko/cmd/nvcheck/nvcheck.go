package nvcheckcmd

import (
	"fmt"

	"github.com/spf13/cobra"

	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	"github.com/Hayao0819/Kamisato/miko/app"
	mikoconfig "github.com/Hayao0819/Kamisato/miko/config"
)

type nvcheckRow struct {
	Pkgbase   string `json:"pkgbase"`
	Published string `json:"published"`
	Upstream  string `json:"upstream"`
	Status    string `json:"status"`
}

const nvcheckDefaultFmt = "table {{.Pkgbase}}\t{{.Published}}\t{{.Upstream}}\t{{.Status}}"

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nvcheck",
		Short: "Check monitored packages for newer upstream versions",
		Args:  cmdline.NoArgs,
		Long: "Fetch the latest upstream version of every entry under nvcheck.entries " +
			"and compare it against the version published on ayato. This is a read-only " +
			"report: it does not enqueue rebuilds (the running server does that on its " +
			"nvcheck.interval_min ticker). Exits non-zero when any entry is out of date.",
		RunE: func(cmd *cobra.Command, args []string) error {
			configFile, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			cfg, err := mikoconfig.LoadMikoConfig(cmd.Flags(), configFile)
			if err != nil {
				return err
			}
			results, err := app.CheckUpstreamVersions(cmd.Context(), cfg)
			if err != nil {
				return err
			}

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
			format, err := cmdline.ResolveFormat(cmd, nvcheckDefaultFmt)
			if err != nil {
				return err
			}
			header := nvcheckRow{Pkgbase: "PKGBASE", Published: "PUBLISHED", Upstream: "UPSTREAM", Status: "STATUS"}
			if err := cmdline.RenderList(cmd.OutOrStdout(), format, header, rows); err != nil {
				return err
			}

			if outdated > 0 {
				return fmt.Errorf("%d package(s) out of date", outdated)
			}
			return nil
		},
	}
	cmdline.AddFormatFlags(cmd)
	return cmd
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
