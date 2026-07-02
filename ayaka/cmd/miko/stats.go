package mikocmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/ayatoclient"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/spf13/cobra"
)

func mikoStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show build service statistics",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := shared.ServerFromFlag(cmd)
			if err != nil {
				return err
			}

			stats, err := ayatoclient.FetchStats(cmd.Context(), srv.URL)
			if err != nil {
				return utils.WrapErr(err, "failed to get stats")
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintf(w, "Workers:\t%d\n", stats.Workers)
			fmt.Fprintf(w, "Queue:\t%d\n", stats.QueueLength)
			fmt.Fprintf(w, "Running:\t%d\n", stats.Running)
			fmt.Fprintf(w, "Total:\t%d\n", stats.Total)
			fmt.Fprintf(w, "Success rate:\t%.1f%%\n", stats.SuccessRate*100)
			fmt.Fprintf(w, "Uptime:\t%s\n", (time.Duration(stats.UptimeSec) * time.Second).String())
			return w.Flush()
		},
	}
}
