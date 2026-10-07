package mikocmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cli"
	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
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

			format, err := cmdline.ResolveFormat(cmd, cli.JobTableFormat)
			if err != nil {
				return err
			}
			return cmdline.RenderList(cmd.OutOrStdout(), format, cli.JobHeader, jobs)
		},
	}
	cmdline.AddFormatFlags(cmd)
	return cmd
}
