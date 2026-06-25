package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	blinky_util "github.com/BrenekH/blinky/cmd/blinky/util"
	"github.com/Hayao0819/Kamisato/internal/ayatoclient"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/spf13/cobra"
)

func mikoJobsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "jobs",
		Short: "List build jobs on miko",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			base, err := resolveJobBase(cmd)
			if err != nil {
				return err
			}

			jobs, err := ayatoclient.ListJobs(base)
			if err != nil {
				return utils.WrapErr(err, "failed to list jobs")
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tREPO\tARCH\tSTATUS\tCREATED")
			for _, j := range jobs {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", j.ID, j.Repo, j.Arch, j.Status, j.CreatedAt)
			}
			return w.Flush()
		},
	}
}

func mikoStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <id>",
		Short: "Show the status of a build job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			base, err := resolveJobBase(cmd)
			if err != nil {
				return err
			}

			job, err := ayatoclient.JobStatus(base, args[0])
			if err != nil {
				return utils.WrapErr(err, "failed to get job status")
			}

			out, err := json.MarshalIndent(job, "", "  ")
			if err != nil {
				return utils.WrapErr(err, "failed to encode job")
			}
			fmt.Println(string(out))
			return nil
		},
	}
}

func mikoLogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs <id>",
		Short: "Stream logs from a build job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			base, err := resolveJobBase(cmd)
			if err != nil {
				return err
			}

			if err := ayatoclient.StreamLogs(base, args[0], os.Stdout); err != nil {
				return utils.WrapErr(err, "failed to stream logs")
			}
			return nil
		},
	}
}

func mikoCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <id>",
		Short: "Cancel a queued or running build job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			server, err := cmd.Flags().GetString("server")
			if err != nil {
				return err
			}

			srv, err := resolveAyatoServer(server)
			if err != nil {
				return err
			}

			if err := ayatoclient.CancelJob(srv.URL, srv.Password, args[0]); err != nil {
				return utils.WrapErr(err, "failed to cancel job")
			}

			fmt.Printf("cancelled job %s\n", args[0])
			return nil
		},
	}
}

func mikoStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show build service statistics",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			base, err := resolveJobBase(cmd)
			if err != nil {
				return err
			}

			stats, err := ayatoclient.FetchStats(base)
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

// resolveJobBase returns the ayato base URL for the job endpoints, which are
// public and need no credentials. It honors the miko --server flag and falls
// back to the serverdb default.
func resolveJobBase(cmd *cobra.Command) (string, error) {
	server, err := cmd.Flags().GetString("server")
	if err != nil {
		return "", err
	}

	db, err := blinky_util.ReadServerDB()
	if err != nil {
		return "", utils.WrapErr(err, "failed to read server database")
	}
	if server == "" {
		server = db.DefaultServer
	}
	if server == "" {
		return "", ErrNoServerSpecified
	}
	return server, nil
}
