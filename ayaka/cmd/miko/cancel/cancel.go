package cancelcmd

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/remote"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <job-id>",
		Short: "Cancel a queued or running build job",
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
			if err := api.CancelJob(cmd.Context(), args[0]); err != nil {
				return errors.WrapErr(err, "failed to cancel job")
			}

			fmt.Fprintf(cmd.OutOrStdout(), "cancelled job %s\n", args[0])
			return nil
		},
	}
}
