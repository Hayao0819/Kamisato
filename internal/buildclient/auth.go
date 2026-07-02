package buildclient

import (
	"context"
	"net/http"
	"strconv"
)

// ExchangeCLICode trades a one-time CLI code plus its PKCE verifier for a CLI
// token over ayaka's direct ayato connection.
func ExchangeCLICode(ctx context.Context, base, code, verifier string) (token, login string, id int64, err error) {
	reqBody := struct {
		Code         string `json:"code"`
		CodeVerifier string `json:"code_verifier"`
	}{Code: code, CodeVerifier: verifier}

	var out struct {
		Token string `json:"token"`
		Login string `json:"login"`
		ID    int64  `json:"id"`
	}
	if err := doJSON(ctx, http.MethodPost, endpoint(base, "/api/unstable/auth/cli/exchange"), "", reqBody, &out, http.StatusOK, "cli exchange"); err != nil {
		return "", "", 0, err
	}
	return out.Token, out.Login, out.ID, nil
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

// RevokeCLIToken denylists the given CLI token server-side by its jti. The token
// authorizes its own revocation, so it is sent as the Bearer credential.
func RevokeCLIToken(ctx context.Context, base, token string) error {
	return doJSON(ctx, http.MethodPost, endpoint(base, "/api/unstable/auth/cli/revoke"), token, nil, nil, http.StatusOK, "revoke token")
}

// RemoveAdmin removes an admin by numeric id.
func RemoveAdmin(ctx context.Context, base, token string, id int64) error {
	return doJSON(ctx, http.MethodDelete, endpoint(base, "/api/unstable/auth/admins/"+strconv.FormatInt(id, 10)), token, nil, nil, http.StatusOK, "remove admin")
}
