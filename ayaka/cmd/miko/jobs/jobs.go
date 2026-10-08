package jobscmd

import (
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/remote"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/miko/internal/joboutput"
	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "List build jobs on miko",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := remote.DefaultProvider().ServerFromFlag(cmd)
			if err != nil {
				return err
			}

			api, err := remote.DefaultProvider().StoredClient(srv)
			if err != nil {
				return err
			}
			jobs, err := api.ListJobs(cmd.Context())
			if err != nil {
				return errors.WrapErr(err, "failed to list jobs")
			}

			format, err := cmdline.ResolveFormat(cmd, joboutput.TableFormat)
			if err != nil {
				return err
			}
			return cmdline.RenderList(cmd.OutOrStdout(), format, joboutput.Header, jobs)
		},
	}
	cmdline.AddFormatFlags(cmd)
	return cmd
}
