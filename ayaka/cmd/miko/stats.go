package mikocmd

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/buildclient"
	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

func mikoStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show build service statistics",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := shared.ServerFromFlag(cmd)
			if err != nil {
				return err
			}

			stats, err := buildclient.FetchStats(cmd.Context(), srv.URL, srv.Password)
			if err != nil {
				return errors.WrapErr(err, "failed to get stats")
			}

			format, err := cliutil.ResolveFormat(cmd, "")
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			// stats is a single aggregate record, so the default is a readable
			// key/value layout; --json / --format keep it scriptable.
			switch {
			case format == "json":
				b, err := json.MarshalIndent(stats, "", "  ")
				if err != nil {
					return errors.WrapErr(err, "failed to encode stats")
				}
				fmt.Fprintln(out, string(b))
				return nil
			case format != "":
				return cliutil.RenderList(out, format, buildclient.Stats{}, []buildclient.Stats{*stats})
			}

			w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
			fmt.Fprintf(w, "Workers:\t%d\n", stats.Workers)
			fmt.Fprintf(w, "Queue:\t%d\n", stats.QueueLength)
			fmt.Fprintf(w, "Running:\t%d\n", stats.Running)
			fmt.Fprintf(w, "Total:\t%d\n", stats.Total)
			fmt.Fprintf(w, "Success rate:\t%.1f%%\n", stats.SuccessRate*100)
			fmt.Fprintf(w, "Uptime:\t%s\n", (time.Duration(stats.UptimeSec) * time.Second).String())
			return w.Flush()
		},
	}
	cliutil.AddFormatFlags(cmd)
	return cmd
}
