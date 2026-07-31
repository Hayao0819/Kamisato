package mikocmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cli"
	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/mikoapi"
)

func mikoStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <job-id>",
		Short: "Show the status of a build job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := cli.ServerFromFlag(cmd)
			if err != nil {
				return err
			}

			api, err := cli.AyatoClient(srv)
			if err != nil {
				return err
			}
			job, err := api.JobStatus(cmd.Context(), args[0])
			if err != nil {
				return errors.WrapErr(err, "failed to get job status")
			}

			// status shows a single job as one row of the same table as `jobs`;
			// --json / --format reach the full record for scripting.
			format, err := cliutil.ResolveFormat(cmd, cli.JobTableFormat)
			if err != nil {
				return err
			}
			return cliutil.RenderList(cmd.OutOrStdout(), format, cli.JobHeader, []mikoapi.Job{*job})
		},
	}
	cliutil.AddFormatFlags(cmd)
	return cmd
}
