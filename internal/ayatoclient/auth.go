package ayatoclient

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

// ExchangeCLICode trades a one-time CLI code plus its PKCE verifier for a CLI
// token over ayaka's direct ayato connection. It returns the issued token and
// the resolved GitHub identity.
func ExchangeCLICode(base, code, verifier string) (token, login string, id int64, err error) {
	body, err := json.Marshal(struct {
		Code         string `json:"code"`
		CodeVerifier string `json:"code_verifier"`
	}{Code: code, CodeVerifier: verifier})
	if err != nil {
		return "", "", 0, utils.WrapErr(err, "failed to encode exchange request")
	}

	resp, err := http.Post(endpoint(base, "/api/unstable/auth/cli/exchange"), "application/json", bytes.NewReader(body))
	if err != nil {
		return "", "", 0, utils.WrapErr(err, "failed to exchange code")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", 0, responseErr(resp, "cli exchange")
	}

	var out struct {
		Token string `json:"token"`
		Login string `json:"login"`
		ID    int64  `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", 0, utils.WrapErr(err, "failed to decode exchange response")
	}
	return out.Token, out.Login, out.ID, nil
}

// ListAdmins fetches the ayato admin allowlist with a Bearer CLI token.
func ListAdmins(base, token string) ([]Admin, error) {
	httpReq, err := http.NewRequest(http.MethodGet, endpoint(base, "/api/unstable/auth/admins"), nil)
	if err != nil {
		return nil, utils.WrapErr(err, "failed to create list admins request")
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, utils.WrapErr(err, "failed to list admins")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, responseErr(resp, "list admins")
	}

	var out struct {
		Admins []Admin `json:"admins"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, utils.WrapErr(err, "failed to decode admins")
	}
	return out.Admins, nil
}

// AddAdmin adds an admin by numeric id or by GitHub login. When id is zero the
// login is sent and ayato resolves it; otherwise the id is sent.
func AddAdmin(base, token string, id int64, login string) (Admin, error) {
	var payload struct {
		ID    int64  `json:"id,omitempty"`
		Login string `json:"login,omitempty"`
	}
	if id == 0 {
		payload.Login = login
	} else {
		payload.ID = id
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Admin{}, utils.WrapErr(err, "failed to encode add admin request")
	}

	httpReq, err := http.NewRequest(http.MethodPost, endpoint(base, "/api/unstable/auth/admins"), bytes.NewReader(body))
	if err != nil {
		return Admin{}, utils.WrapErr(err, "failed to create add admin request")
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return Admin{}, utils.WrapErr(err, "failed to add admin")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Admin{}, responseErr(resp, "add admin")
	}

	var admin Admin
	if err := json.NewDecoder(resp.Body).Decode(&admin); err != nil {
		return Admin{}, utils.WrapErr(err, "failed to decode admin")
	}
	return admin, nil
}

// RemoveAdmin removes an admin by numeric id.
func RemoveAdmin(base, token string, id int64) error {
	httpReq, err := http.NewRequest(http.MethodDelete, endpoint(base, "/api/unstable/auth/admins/"+strconv.FormatInt(id, 10)), nil)
	if err != nil {
		return utils.WrapErr(err, "failed to create remove admin request")
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return utils.WrapErr(err, "failed to remove admin")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return responseErr(resp, "remove admin")
	}
	return nil
}
