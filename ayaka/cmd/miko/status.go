package mikocmd

import (
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/ayatoclient"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/spf13/cobra"
)

func mikoStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <id>",
		Short: "Show the status of a build job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := shared.ServerFromFlag(cmd)
			if err != nil {
				return err
			}

			job, err := ayatoclient.JobStatus(cmd.Context(), srv.URL, srv.Password, args[0])
			if err != nil {
				return errwrap.WrapErr(err, "failed to get job status")
			}

			// status shows a single job as one row of the same table as `jobs`;
			// --json / --format reach the full record for scripting.
			format, err := shared.ResolveFormat(cmd, jobTableFormat)
			if err != nil {
				return err
			}
			return shared.RenderList(cmd.OutOrStdout(), format, jobHeader, []ayatoclient.Job{*job})
		},
	}
	shared.AddFormatFlags(cmd)
	return cmd
}
