package admincmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cli"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

func adminRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <login-or-id>",
		Short: "Remove an ayato admin by GitHub login or numeric id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := cli.ServerFromFlag(cmd)
			if err != nil {
				return err
			}
			api, err := cli.AyatoClient(srv)
			if err != nil {
				return err
			}
			removed, err := resolveAdminID(cmd.Context(), api, args[0])
			if err == nil {
				err = api.RemoveAdmin(cmd.Context(), removed)
			}
			if err != nil {
				return errors.WrapErr(err, "failed to remove admin")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed admin %d\n", removed)
			return nil
		},
	}
}
