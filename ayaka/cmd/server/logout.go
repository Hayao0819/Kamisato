package servercmd

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/spf13/cobra"
)

// LogoutCmd clears the locally stored CLI token but keeps the server registered.
// It only clears local state; the token keeps working elsewhere until TTL expiry,
// allowlist removal, or signer rotation. Use `server revoke` to invalidate the
// token server-side.
func LogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout <server>",
		Short: "Clear the stored login for an ayato server",
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

			entry.Username = ""
			entry.Password = ""
			db.Servers[server] = entry
			blinkyutils.ForgetSecret(server)
			blinkyutils.ForgetRefreshSecret(server)

			if err := blinkyutils.SaveServerDB(db); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Logged out")
			return nil
		},
	}
}
