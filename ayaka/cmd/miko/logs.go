package mikocmd

import (
	"os"

	"github.com/Hayao0819/Kamisato/internal/ayatoclient"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/spf13/cobra"
)

func mikoLogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs <id>",
		Short: "Stream logs from a build job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			base, err := resolveJobBase(cmd)
			if err != nil {
				return err
			}

			if err := ayatoclient.StreamLogs(cmd.Context(), base, args[0], os.Stdout); err != nil {
				return utils.WrapErr(err, "failed to stream logs")
			}
			return nil
		},
	}
}
