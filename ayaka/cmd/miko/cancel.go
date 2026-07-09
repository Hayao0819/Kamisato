package mikocmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/buildclient"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

func mikoCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <job-id>",
		Short: "Cancel a queued or running build job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := shared.ServerFromFlag(cmd)
			if err != nil {
				return err
			}

			if err := buildclient.CancelJob(cmd.Context(), srv.URL, srv.Password, args[0]); err != nil {
				return errors.WrapErr(err, "failed to cancel job")
			}

			fmt.Fprintf(cmd.OutOrStdout(), "cancelled job %s\n", args[0])
			return nil
		},
	}
}
