package mikocmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cli"
	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

func mikoJobsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "List build jobs on miko",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := cli.ServerFromFlag(cmd)
			if err != nil {
				return err
			}

			api, err := cli.AyatoClient(srv)
			if err != nil {
				return err
			}
			jobs, err := api.ListJobs(cmd.Context())
			if err != nil {
				return errors.WrapErr(err, "failed to list jobs")
			}

			format, err := cliutil.ResolveFormat(cmd, cli.JobTableFormat)
			if err != nil {
				return err
			}
			return cliutil.RenderList(cmd.OutOrStdout(), format, cli.JobHeader, jobs)
		},
	}
	cliutil.AddFormatFlags(cmd)
	return cmd
}
