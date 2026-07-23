package admincmd

import (
	"context"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/client"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Manage ayato admin allowlist",
		Long:  "List, add, and remove ayato admins. Requires a logged-in server with a CLI token.",
	}
	shared.AddPersistentServerFlag(cmd)

	cmd.AddCommand(adminListCmd(), adminAddCmd(), adminRemoveCmd())
	return cmd
}

// parseLoginOrID splits a "login-or-id" argument: a positive integer is a
// numeric GitHub user ID; anything else is a GitHub login name.
func parseLoginOrID(s string) (id int64, login string) {
	if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
		return n, ""
	}
	return 0, s
}

// resolveAdminID returns the numeric GitHub user ID for s. If s is a positive
// integer it is returned as-is; otherwise admins are listed to find the login.
func resolveAdminID(ctx context.Context, api *client.Ayato, s string) (int64, error) {
	if id, _ := parseLoginOrID(s); id > 0 {
		return id, nil
	}
	_, login := parseLoginOrID(s)
	admins, err := api.ListAdmins(ctx)
	if err != nil {
		return 0, errors.WrapErr(err, "failed to list admins for login resolution")
	}
	for _, a := range admins {
		if a.Login == login {
			return a.ID, nil
		}
	}
	return 0, errors.NewErrf("no admin with login %q", login)
}
