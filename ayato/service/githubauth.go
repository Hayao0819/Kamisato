package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// ResolveGitHubLogin resolves a GitHub login to its numeric id and canonical
// login via the public users API. No auth is needed: public GitHub profiles are
// world-readable. It lives in the service so the handler never makes the outbound
// call.
func (s *Service) ResolveGitHubLogin(ctx context.Context, login string) (int64, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	u := "https://api.github.com/users/" + url.PathEscape(login)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, "", fmt.Errorf("github users lookup non-200: %d", resp.StatusCode)
	}
	var gu struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&gu); err != nil || gu.ID == 0 {
		return 0, "", fmt.Errorf("github users decode")
	}
	return gu.ID, gu.Login, nil
}
