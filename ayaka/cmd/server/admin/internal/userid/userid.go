package userid

import (
	"context"
	"strconv"

	"github.com/Hayao0819/Kamisato/ayato/client"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

// Parse accepts a positive numeric ID or a GitHub login.
func Parse(value string) (int64, string) {
	if id, err := strconv.ParseInt(value, 10, 64); err == nil && id > 0 {
		return id, ""
	}
	return 0, value
}

func Resolve(ctx context.Context, api *client.Client, value string) (int64, error) {
	id, login := Parse(value)
	if id > 0 {
		return id, nil
	}
	admins, err := api.ListAdmins(ctx)
	if err != nil {
		return 0, errors.WrapErr(err, "failed to list admins for login resolution")
	}
	for _, admin := range admins {
		if admin.Login == login {
			return admin.ID, nil
		}
	}
	return 0, errors.NewErrf("no admin with login %q", login)
}
