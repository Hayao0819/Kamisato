package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/auth"
)

func postRefresh(t *testing.T, h *Handler, refreshToken string) *httptest.ResponseRecorder {
	t.Helper()
	r := gin.New()
	r.POST("/refresh", h.RefreshHandler)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/refresh", strings.NewReader(`{"refresh_token":"`+refreshToken+`"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestRefreshTokenHasAtMostOneConcurrentWinner(t *testing.T) {
	handler, _, _ := denylistHandler(t, 42)
	_, refresh, _, err := handler.issueAccessRefresh(42, "alice", "cli")
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
	h, dl, _ := denylistHandler(t, 42)

	_, refresh, _, err := h.issueAccessRefresh(42, "alice", "cli")
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

func TestRevokeOldPairInvalidatesAlreadyRotatedSession(t *testing.T) {
	h, dl, _ := denylistHandler(t, 42)
	oldAccess, oldRefresh, _, err := h.issueAccessRefresh(42, "alice", "cli")
	if err != nil {
		t.Fatal(err)
	}

	refreshResponse := postRefresh(t, h, oldRefresh)
	if refreshResponse.Code != http.StatusOK {
		t.Fatalf("refresh = %d, body %s", refreshResponse.Code, refreshResponse.Body.String())
	}
	var rotated struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(refreshResponse.Body.Bytes(), &rotated); err != nil {
		t.Fatal(err)
	}
	family := claimsOf(t, rotated.Token).SessionID
	if family == "" {
		t.Fatal("rotated access token has no session family")
	}

	if w := postCLIRevoke(h, oldAccess, oldRefresh); w.Code != http.StatusOK {
		t.Fatalf("revoke old pair = %d, body %s", w.Code, w.Body.String())
	}
	if revoked, err := dl.IsSessionRevoked(family); err != nil || !revoked {
		t.Fatalf("session family revoke = (%v, %v), want true, nil", revoked, err)
	}

	r := gin.New()
	r.GET("/me", h.MeHandler)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+rotated.Token)
	r.ServeHTTP(w, req)
	var me struct {
		Authenticated bool `json:"authenticated"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &me); err != nil {
		t.Fatal(err)
	}
	if me.Authenticated {
		t.Fatal("rotated access token authenticated after family revoke")
	}
	if w := postRefresh(t, h, rotated.RefreshToken); w.Code != http.StatusUnauthorized {
		t.Fatalf("rotated refresh after family revoke = %d, want 401", w.Code)
	}
}

// A denylisted refresh token is rejected even before its TTL elapses.
func TestRefreshRejectsDenylisted(t *testing.T) {
	h, dl, _ := denylistHandler(t, 42)
	_, refresh, _, err := h.issueAccessRefresh(42, "alice", "cli")
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
	h, _, signer := denylistHandler(t, 42)
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

func TestLegacyRefreshAdoptsItsJTIAsSessionFamily(t *testing.T) {
	h, dl, signer := denylistHandler(t, 42)
	legacy, err := signer.Sign(auth.Claims{
		Typ: auth.TypRefresh, GitHubID: 42, Login: "alice", JTI: "legacy-refresh-jti", Exp: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	w := postRefresh(t, h, legacy)
	if w.Code != http.StatusOK {
		t.Fatalf("legacy refresh = %d, body %s", w.Code, w.Body.String())
	}
	var pair struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &pair); err != nil {
		t.Fatal(err)
	}
	if got := claimsOf(t, pair.Token).SessionID; got != "legacy-refresh-jti" {
		t.Fatalf("legacy access family = %q", got)
	}
	if got := claimsOf(t, pair.RefreshToken).SessionID; got != "legacy-refresh-jti" {
		t.Fatalf("legacy refresh family = %q", got)
	}
	if revoke := postCLIRevoke(h, "", legacy); revoke.Code != http.StatusOK {
		t.Fatalf("revoke legacy ancestor = %d, body %s", revoke.Code, revoke.Body.String())
	}
	if revoked, err := dl.IsSessionRevoked("legacy-refresh-jti"); err != nil || !revoked {
		t.Fatalf("legacy family revoke = (%v, %v)", revoked, err)
	}
}
