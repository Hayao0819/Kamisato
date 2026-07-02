package mikocmd

import (
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/ayatoclient"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/spf13/cobra"
)

func mikoJobsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "List build jobs on miko",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := shared.ServerFromFlag(cmd)
			if err != nil {
				return err
			}

			jobs, err := ayatoclient.ListJobs(cmd.Context(), srv.URL, srv.Password)
			if err != nil {
				return errwrap.WrapErr(err, "failed to list jobs")
			}

			format, err := shared.ResolveFormat(cmd, jobTableFormat)
			if err != nil {
				return err
			}
			return shared.RenderList(cmd.OutOrStdout(), format, jobHeader, jobs)
		},
	}
	shared.AddFormatFlags(cmd)
	return cmd
}
