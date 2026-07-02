package mikocmd

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/ayatoclient"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/spf13/cobra"
)

func mikoCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <id>",
		Short: "Cancel a queued or running build job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := shared.ServerFromFlag(cmd)
			if err != nil {
				return err
			}

			if err := ayatoclient.CancelJob(cmd.Context(), srv.URL, srv.Password, args[0]); err != nil {
				return utils.WrapErr(err, "failed to cancel job")
			}

			fmt.Printf("cancelled job %s\n", args[0])
			return nil
		},
	}
}
