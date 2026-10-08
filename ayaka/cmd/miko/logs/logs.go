package logscmd

import (
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/remote"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs <job-id>",
		Short: "Stream logs from a build job",
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
			if err := api.StreamLogs(cmd.Context(), args[0], cmd.OutOrStdout()); err != nil {
				return errors.WrapErr(err, "failed to stream logs")
			}
			return nil
		},
	}
}
