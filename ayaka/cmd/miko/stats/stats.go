package statscmd

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/remote"
	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	"github.com/Hayao0819/Kamisato/internal/errors"
	miko "github.com/Hayao0819/Kamisato/miko/client"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show build service statistics",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := remote.DefaultProvider().ServerFromFlag(cmd)
			if err != nil {
				return err
			}

			api, err := remote.DefaultProvider().StoredClient(srv)
			if err != nil {
				return err
			}
			stats, err := api.FetchStats(cmd.Context())
			if err != nil {
				return errors.WrapErr(err, "failed to get stats")
			}

			format, err := cmdline.ResolveFormat(cmd, "")
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
				return cmdline.RenderList(out, format, miko.Stats{}, []miko.Stats{*stats})
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
	cmdline.AddFormatFlags(cmd)
	return cmd
}
