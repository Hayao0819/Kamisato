package mikocmd

import (
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/buildclient"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/spf13/cobra"
)

func mikoLogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs <job-id>",
		Short: "Stream logs from a build job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := shared.ServerFromFlag(cmd)
			if err != nil {
				return err
			}

			if err := buildclient.StreamLogs(cmd.Context(), srv.URL, srv.Password, args[0], cmd.OutOrStdout()); err != nil {
				return errwrap.WrapErr(err, "failed to stream logs")
			}
			return nil
		},
	}
}
