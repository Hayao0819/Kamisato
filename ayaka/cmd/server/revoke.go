package servercmd

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/ayatoclient"
	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/spf13/cobra"
)

// RevokeCmd invalidates the stored CLI token server-side (via the denylist) and
// then clears it locally. Unlike logout, this stops the token working on every
// replica immediately, not just on this machine.
func RevokeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <server_url>",
		Short: "Revoke the stored CLI token on an ayato server and clear it locally",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			server := args[0]

			db, err := blinkyutils.ReadServerDB()
			if err != nil {
				return err
			}

			entry, ok := db.Servers[server]
			if !ok {
				return errwrap.WrapErr(shared.ErrServerNotFound, server)
			}
			if entry.Password == "" {
				return errwrap.NewErr("no stored token to revoke for " + server)
			}

			if err := ayatoclient.RevokeCLIToken(cmd.Context(), server, entry.Password); err != nil {
				return errwrap.WrapErr(err, "failed to revoke token")
			}

			entry.Username = ""
			entry.Password = ""
			db.Servers[server] = entry
			if err := blinkyutils.SaveServerDB(db); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "トークンを失効しました")
			return nil
		},
	}
}
