package removecmd

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/remote"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/server/admin/internal/userid"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <login-or-id>",
		Short: "Remove an ayato admin by GitHub login or numeric id",
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
			removed, err := userid.Resolve(cmd.Context(), api, args[0])
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
