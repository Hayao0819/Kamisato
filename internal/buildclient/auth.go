package buildclient

import (
	"context"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

func ExchangeCLICode(ctx context.Context, base, code, verifier string) (token, refresh, login string, id int64, err error) {
	c, err := ayato(base, "")
	if err != nil {
		return "", "", "", 0, err
	}
	result, err := c.ExchangeCLICode(ctx, code, verifier)
	if err != nil {
		return "", "", "", 0, err
	}
	return result.AccessToken, result.RefreshToken, result.Login, result.ID, nil
}

func RefreshAccessToken(ctx context.Context, base, refresh string) (token, newRefresh, login string, id int64, err error) {
	c, err := ayato(base, "")
	if err != nil {
		return "", "", "", 0, err
	}
	result, err := c.RefreshAccessToken(ctx, refresh)
	if err != nil {
		return "", "", "", 0, err
	}
	return result.AccessToken, result.RefreshToken, result.Login, result.ID, nil
}

// WithRefresh configures credential refresh for compatibility callers.
func WithRefresh(
	ctx context.Context,
	base, access, refresh string,
	persist func(access, refresh string) error,
	op func(context.Context, string) error,
) error {
	err := op(ctx, access)
	if !errors.Is(err, ErrAccessTokenExpired) || refresh == "" {
		return err
	}
	newAccess, newRefresh, _, _, refreshErr := RefreshAccessToken(ctx, base, refresh)
	if refreshErr != nil {
		return errors.WrapErr(refreshErr, "session expired; please run 'ayaka server login' again")
	}
	if persist != nil {
		if err := persist(newAccess, newRefresh); err != nil {
			return errors.WrapErr(err, "save refreshed tokens")
		}
	}
	return op(ctx, newAccess)
}

func ListAdmins(ctx context.Context, base, token string) ([]Admin, error) {
	c, err := ayato(base, token)
	if err != nil {
		return nil, err
	}
	return c.ListAdmins(ctx)
}

func AddAdmin(ctx context.Context, base, token string, id int64, login string) (Admin, error) {
	c, err := ayato(base, token)
	if err != nil {
		return Admin{}, err
	}
	return c.AddAdmin(ctx, id, login)
}

func RevokeCLIToken(ctx context.Context, base, token, refresh string) error {
	c, err := ayato(base, token)
	if err != nil {
		return err
	}
	return c.RevokeCLIToken(ctx, refresh)
}

func RemoveAdmin(ctx context.Context, base, token string, id int64) error {
	c, err := ayato(base, token)
	if err != nil {
		return err
	}
	return c.RemoveAdmin(ctx, id)
}
