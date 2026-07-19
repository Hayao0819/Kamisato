package servercmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/serverstore"
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

			if err := serverstore.ClearCredentials(server, true); err != nil {
				return errors.WrapErr(err, "local credential deletion failed; retry logout")
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Logged out")
			return nil
		},
	}
}
