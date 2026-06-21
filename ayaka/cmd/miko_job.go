package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	blinky_util "github.com/BrenekH/blinky/cmd/blinky/util"
	"github.com/Hayao0819/Kamisato/internal/ayatoclient"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/spf13/cobra"
)

// mikoJobsCmd lists the build jobs known to miko.
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

// mikoStatusCmd prints the status of a single build job.
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

// mikoLogsCmd streams the live build logs of a job.
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
		return "", utils.NewErr("no server specified and no default server is set")
	}
	return server, nil
}
