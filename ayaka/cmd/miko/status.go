package mikocmd

import (
	"encoding/json"
	"fmt"

	"github.com/Hayao0819/Kamisato/internal/ayatoclient"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/spf13/cobra"
)

func mikoStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <id>",
		Short: "Show the status of a build job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			base, err := resolveJobBase(cmd)
			if err != nil {
				return err
			}

			job, err := ayatoclient.JobStatus(cmd.Context(), base, args[0])
			if err != nil {
				return utils.WrapErr(err, "failed to get job status")
			}

			out, err := json.MarshalIndent(job, "", "  ")
			if err != nil {
				return utils.WrapErr(err, "failed to encode job")
			}
			fmt.Println(string(out))
			return nil
		},
	}
}
