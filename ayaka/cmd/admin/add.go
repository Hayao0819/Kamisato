package admincmd

import (
	"fmt"
	"strconv"

	"github.com/Hayao0819/Kamisato/internal/ayatoclient"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/spf13/cobra"
)

func adminAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <login-or-id>",
		Short: "Add an ayato admin by GitHub login or numeric id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := resolveAdminServer(cmd)
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
			admin, err := ayatoclient.AddAdmin(srv.URL, srv.Password, id, login)
			if err != nil {
				return utils.WrapErr(err, "failed to add admin")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added %s (%d)\n", admin.Login, admin.ID)
			return nil
		},
	}
}
