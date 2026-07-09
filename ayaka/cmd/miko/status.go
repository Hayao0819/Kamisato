package mikocmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/buildclient"
	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

func mikoStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <job-id>",
		Short: "Show the status of a build job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := shared.ServerFromFlag(cmd)
			if err != nil {
				return err
			}

			job, err := buildclient.JobStatus(cmd.Context(), srv.URL, srv.Password, args[0])
			if err != nil {
				return errors.WrapErr(err, "failed to get job status")
			}

			// status shows a single job as one row of the same table as `jobs`;
			// --json / --format reach the full record for scripting.
			format, err := cliutil.ResolveFormat(cmd, jobTableFormat)
			if err != nil {
				return err
			}
			return cliutil.RenderList(cmd.OutOrStdout(), format, jobHeader, []buildclient.Job{*job})
		},
	}
	cliutil.AddFormatFlags(cmd)
	return cmd
}
