package buildclient

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/Hayao0819/Kamisato/internal/errwrap"
)

// ExchangeCLICode trades a one-time CLI code plus its PKCE verifier for an access
// token and its paired refresh token over ayaka's direct ayato connection.
func ExchangeCLICode(ctx context.Context, base, code, verifier string) (token, refresh, login string, id int64, err error) {
	reqBody := struct {
		Code         string `json:"code"`
		CodeVerifier string `json:"code_verifier"`
	}{Code: code, CodeVerifier: verifier}

	var out struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
		Login        string `json:"login"`
		ID           int64  `json:"id"`
	}
	if err := doJSON(ctx, http.MethodPost, endpoint(base, "/api/unstable/auth/cli/exchange"), "", reqBody, &out, http.StatusOK, "cli exchange"); err != nil {
		return "", "", "", 0, err
	}
	return out.Token, out.RefreshToken, out.Login, out.ID, nil
}

// RefreshAccessToken trades a refresh token for a fresh access token (and a
// rotated refresh token) at ayato's refresh endpoint. It needs no bearer — the
// refresh token in the body is the credential.
func RefreshAccessToken(ctx context.Context, base, refresh string) (token, newRefresh, login string, id int64, err error) {
	reqBody := struct {
		RefreshToken string `json:"refresh_token"`
	}{RefreshToken: refresh}

	var out struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
		Login        string `json:"login"`
		ID           int64  `json:"id"`
	}
	if err := doJSON(ctx, http.MethodPost, endpoint(base, "/api/unstable/auth/refresh"), "", reqBody, &out, http.StatusOK, "token refresh"); err != nil {
		return "", "", "", 0, err
	}
	return out.Token, out.RefreshToken, out.Login, out.ID, nil
}

// WithRefresh runs op with the access token; if op fails only because the token
// expired, it trades the refresh token for a new pair, persists it, and retries op
// once. A failed refresh surfaces a re-login error; with no refresh token it just
// runs op once.
func WithRefresh(ctx context.Context, base, access, refresh string, persist func(access, refresh string) error, op func(ctx context.Context, token string) error) error {
	err := op(ctx, access)
	if !errors.Is(err, ErrAccessTokenExpired) || refresh == "" {
		return err
	}
	newAccess, newRefresh, _, _, rerr := RefreshAccessToken(ctx, base, refresh)
	if rerr != nil {
		return errwrap.WrapErr(rerr, "session expired; please run 'ayaka server login' again")
	}
	if persist != nil {
		if perr := persist(newAccess, newRefresh); perr != nil {
			return errwrap.WrapErr(perr, "failed to save refreshed tokens")
		}
	}
	return op(ctx, newAccess)
}

// ListAdmins fetches the ayato admin allowlist with a Bearer CLI token.
func ListAdmins(ctx context.Context, base, token string) ([]Admin, error) {
	var out struct {
		Admins []Admin `json:"admins"`
	}
	if err := doJSON(ctx, http.MethodGet, endpoint(base, "/api/unstable/auth/admins"), token, nil, &out, http.StatusOK, "list admins"); err != nil {
		return nil, err
	}
	return out.Admins, nil
}

// AddAdmin adds an admin by numeric id or by GitHub login. When id is zero the
// login is sent and ayato resolves it; otherwise the id is sent.
func AddAdmin(ctx context.Context, base, token string, id int64, login string) (Admin, error) {
	var payload struct {
		ID    int64  `json:"id,omitempty"`
		Login string `json:"login,omitempty"`
	}
	if id == 0 {
		payload.Login = login
	} else {
		payload.ID = id
	}

	var admin Admin
	if err := doJSON(ctx, http.MethodPost, endpoint(base, "/api/unstable/auth/admins"), token, payload, &admin, http.StatusOK, "add admin"); err != nil {
		return Admin{}, err
	}
	return admin, nil
}

// RevokeCLIToken denylists the given access token server-side by its jti and, when
// a refresh token is supplied, that too, so a logout kills both halves of the
// session. The tokens authorize their own revocation: the access token rides as the
// Bearer credential and the refresh token in the body.
func RevokeCLIToken(ctx context.Context, base, token, refresh string) error {
	var body any
	if refresh != "" {
		body = struct {
			RefreshToken string `json:"refresh_token"`
		}{RefreshToken: refresh}
	}
	return doJSON(ctx, http.MethodPost, endpoint(base, "/api/unstable/auth/cli/revoke"), token, body, nil, http.StatusOK, "revoke token")
}

// RemoveAdmin removes an admin by numeric id.
func RemoveAdmin(ctx context.Context, base, token string, id int64) error {
	return doJSON(ctx, http.MethodDelete, endpoint(base, "/api/unstable/auth/admins/"+strconv.FormatInt(id, 10)), token, nil, nil, http.StatusOK, "remove admin")
}
