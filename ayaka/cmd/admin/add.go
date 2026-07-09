package admincmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/buildclient"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

func adminAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <login-or-id>",
		Short: "Add an ayato admin by GitHub login or numeric id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := shared.ServerFromFlag(cmd)
			if err != nil {
				return err
			}
			id, login := parseLoginOrID(args[0])
			var admin buildclient.Admin
			err = shared.WithServerAuth(cmd.Context(), srv, func(ctx context.Context, token string) error {
				var aerr error
				admin, aerr = buildclient.AddAdmin(ctx, srv.URL, token, id, login)
				return aerr
			})
			if err != nil {
				return errors.WrapErr(err, "failed to add admin")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added %s (%d)\n", admin.Login, admin.ID)
			return nil
		},
	}
}
