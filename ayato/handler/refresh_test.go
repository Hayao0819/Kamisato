package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/auth"
)

func postRefresh(t *testing.T, h *AuthHandler, refreshToken string) *httptest.ResponseRecorder {
	t.Helper()
	return postJSON(
		t,
		"/refresh",
		`{"refresh_token":"`+refreshToken+`"}`,
		h.RefreshHandler,
	)
}

func TestRefreshTokenHasAtMostOneConcurrentWinner(t *testing.T) {
	handler, _, _ := denylistHandler(t)
	_, refresh, _, err := handler.issueAccessRefresh(42, "alice")
	if err != nil {
		t.Fatal(err)
	}
	const contenders = 8
	start := make(chan struct{})
	statuses := make(chan int, contenders)
	var wait sync.WaitGroup
	for range contenders {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			statuses <- postRefresh(t, handler, refresh).Code
		}()
	}
	close(start)
	wait.Wait()
	close(statuses)
	winners := 0
	for status := range statuses {
		if status == http.StatusOK {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("successful refreshes = %d, want exactly 1", winners)
	}
}

// A valid refresh token mints a fresh access token (with a jti), rotates the
// refresh token, and denylists the consumed one so it cannot be reused.
func TestRefreshIssuesNewAccessAndRotates(t *testing.T) {
	h, dl, _ := denylistHandler(t)

	_, refresh, _, err := h.issueAccessRefresh(42, "alice")
	if err != nil {
		t.Fatalf("issueAccessRefresh: %v", err)
	}
	oldJTI := jtiOf(t, refresh)
	oldClaims := claimsOf(t, refresh)
	if oldClaims.SessionID == "" {
		t.Fatal("new refresh token must carry a session family id")
	}

	w := postRefresh(t, h, refresh)
	if w.Code != http.StatusOK {
		t.Fatalf("refresh = %d, want 200 (body %s)", w.Code, w.Body.String())
	}
	var resp struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Login        string `json:"login"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Login != "alice" || resp.ExpiresIn <= 0 {
		t.Fatalf("resp = %+v, want login alice and a positive expires_in", resp)
	}
	accessClaims, verr := h.signer.VerifyTyp(resp.Token, auth.TypCLI)
	if verr != nil {
		t.Fatalf("issued access token must verify as a CLI token: %v", verr)
	}
	refreshClaims, verr := h.signer.VerifyTyp(resp.RefreshToken, auth.TypRefresh)
	if verr != nil {
		t.Fatalf("rotated refresh token must verify as a refresh token: %v", verr)
	}
	if accessClaims.SessionID != oldClaims.SessionID || refreshClaims.SessionID != oldClaims.SessionID {
		t.Fatalf("rotation changed session family: old=%q access=%q refresh=%q", oldClaims.SessionID, accessClaims.SessionID, refreshClaims.SessionID)
	}
	if jtiOf(t, resp.RefreshToken) == oldJTI {
		t.Fatal("rotation must mint a new refresh jti")
	}
	if revoked, err := dl.IsRevoked(oldJTI); err != nil || !revoked {
		t.Fatal("the consumed refresh token's jti must be denylisted (rotation)")
	}
	// The consumed refresh token is now revoked, so re-using it is rejected.
	if w := postRefresh(t, h, refresh); w.Code != http.StatusUnauthorized {
		t.Fatalf("reused refresh = %d, want 401", w.Code)
	}
}

// A denylisted refresh token is rejected even before its TTL elapses.
func TestRefreshRejectsDenylisted(t *testing.T) {
	h, dl, _ := denylistHandler(t)
	_, refresh, _, err := h.issueAccessRefresh(42, "alice")
	if err != nil {
		t.Fatalf("issueAccessRefresh: %v", err)
	}
	if err := dl.Revoke(jtiOf(t, refresh), time.Hour); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if w := postRefresh(t, h, refresh); w.Code != http.StatusUnauthorized {
		t.Fatalf("denylisted refresh = %d, want 401", w.Code)
	}
}

// An expired refresh token is rejected (the client must re-login).
func TestRefreshRejectsExpired(t *testing.T) {
	h, _, signer := denylistHandler(t)
	expired, err := signer.Sign(auth.Claims{
		Typ:      auth.TypRefresh,
		GitHubID: 42,
		Login:    "alice",
		JTI:      "r-old",
		Exp:      time.Now().Add(-time.Minute),
	})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if w := postRefresh(t, h, expired); w.Code != http.StatusUnauthorized {
		t.Fatalf("expired refresh = %d, want 401", w.Code)
	}
}
