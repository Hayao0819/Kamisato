package servercmd

import (
	"fmt"

	blinky_util "github.com/BrenekH/blinky/cmd/blinky/util"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/spf13/cobra"
)

// LogoutCmd clears the locally stored CLI token but keeps the server registered.
// The token is a stateless signed envelope, so it cannot be revoked server-side;
// it only stops working at TTL expiry, allowlist removal, or signer rotation.
func LogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout <server_url>",
		Short: "Clear the stored login for an ayato server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			server := args[0]

			db, err := blinky_util.ReadServerDB()
			if err != nil {
				return utils.WrapErr(err, "failed to read server database")
			}

			entry, ok := db.Servers[server]
			if !ok {
				return utils.WrapErr(shared.ErrServerNotFound, server)
			}

			entry.Username = ""
			entry.Password = ""
			db.Servers[server] = entry

			if err := blinky_util.SaveServerDB(db); err != nil {
				return utils.WrapErr(err, "failed to save server database")
			}

			fmt.Fprintln(cmd.OutOrStdout(), "ログアウトしました")
			return nil
		},
	}
}
