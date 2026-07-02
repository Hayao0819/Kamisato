package admincmd

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/buildclient"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/spf13/cobra"
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
			var id int64
			var login string
			if n, perr := strconv.ParseInt(args[0], 10, 64); perr == nil && n > 0 {
				id = n
			} else {
				login = args[0]
			}
			var admin buildclient.Admin
			err = shared.WithServerAuth(cmd.Context(), srv, func(ctx context.Context, token string) error {
				var aerr error
				admin, aerr = buildclient.AddAdmin(ctx, srv.URL, token, id, login)
				return aerr
			})
			if err != nil {
				return errwrap.WrapErr(err, "failed to add admin")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added %s (%d)\n", admin.Login, admin.ID)
			return nil
		},
	}
}
