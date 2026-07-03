package admincmd

import (
	"context"
	"fmt"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/buildclient"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/spf13/cobra"
)

func adminRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <login-or-id>",
		Short: "Remove an ayato admin by GitHub login or numeric id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := shared.ServerFromFlag(cmd)
			if err != nil {
				return err
			}
			var removed int64
			err = shared.WithServerAuth(cmd.Context(), srv, func(ctx context.Context, token string) error {
				id, aerr := resolveAdminID(ctx, srv, token, args[0])
				if aerr != nil {
					return aerr
				}
				removed = id
				return buildclient.RemoveAdmin(ctx, srv.URL, token, id)
			})
			if err != nil {
				return errwrap.WrapErr(err, "failed to remove admin")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed admin %d\n", removed)
			return nil
		},
	}
}
