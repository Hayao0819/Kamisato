package mikocmd

import (
	"fmt"
	"os"
	"text/tabwriter"

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

			jobs, err := ayatoclient.ListJobs(cmd.Context(), base)
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
