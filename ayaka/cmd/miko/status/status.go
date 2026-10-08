package statuscmd

import (
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/remote"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/miko/internal/joboutput"
	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	"github.com/Hayao0819/Kamisato/internal/errors"
	miko "github.com/Hayao0819/Kamisato/miko/client"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <job-id>",
		Short: "Show the status of a build job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := remote.DefaultProvider().ServerFromFlag(cmd)
			if err != nil {
				return err
			}

			api, err := remote.DefaultProvider().StoredClient(srv)
			if err != nil {
				return err
			}
			job, err := api.JobStatus(cmd.Context(), args[0])
			if err != nil {
				return errors.WrapErr(err, "failed to get job status")
			}

			// status shows a single job as one row of the same table as `jobs`;
			// --json / --format reach the full record for scripting.
			format, err := cmdline.ResolveFormat(cmd, joboutput.TableFormat)
			if err != nil {
				return err
			}
			return cmdline.RenderList(cmd.OutOrStdout(), format, joboutput.Header, []miko.Job{*job})
		},
	}
	cmdline.AddFormatFlags(cmd)
	return cmd
}
