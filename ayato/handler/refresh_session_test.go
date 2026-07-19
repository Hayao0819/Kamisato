package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/auth"
)

func TestRevokeOldPairInvalidatesAlreadyRotatedSession(t *testing.T) {
	handler, denylist, _ := denylistHandler(t, 42)
	oldAccess, oldRefresh, _, err := handler.issueAccessRefresh(42, "alice", "cli")
	if err != nil {
		t.Fatal(err)
	}

	refreshResponse := postRefresh(t, handler, oldRefresh)
	if refreshResponse.Code != http.StatusOK {
		t.Fatalf("refresh = %d: %s", refreshResponse.Code, refreshResponse.Body.String())
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

	if response := postCLIRevoke(handler, oldAccess, oldRefresh); response.Code != http.StatusOK {
		t.Fatalf("revoke old pair = %d: %s", response.Code, response.Body.String())
	}
	if revoked, err := denylist.IsSessionRevoked(family); err != nil || !revoked {
		t.Fatalf("session family revoke = (%v, %v), want true, nil", revoked, err)
	}

	router := gin.New()
	router.GET("/me", handler.MeHandler)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/me", nil)
	request.Header.Set("Authorization", "Bearer "+rotated.Token)
	router.ServeHTTP(response, request)
	var me struct {
		Authenticated bool `json:"authenticated"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &me); err != nil {
		t.Fatal(err)
	}
	if me.Authenticated {
		t.Fatal("rotated access token authenticated after family revoke")
	}
	if response := postRefresh(t, handler, rotated.RefreshToken); response.Code != http.StatusUnauthorized {
		t.Fatalf("rotated refresh after family revoke = %d, want 401", response.Code)
	}
}

func TestLegacyRefreshAdoptsItsJTIAsSessionFamily(t *testing.T) {
	handler, denylist, signer := denylistHandler(t, 42)
	legacy, err := signer.Sign(auth.Claims{
		Typ:      auth.TypRefresh,
		GitHubID: 42,
		Login:    "alice",
		JTI:      "legacy-refresh-jti",
		Exp:      time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	response := postRefresh(t, handler, legacy)
	if response.Code != http.StatusOK {
		t.Fatalf("legacy refresh = %d: %s", response.Code, response.Body.String())
	}
	var pair struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &pair); err != nil {
		t.Fatal(err)
	}
	for name, token := range map[string]string{
		"access":  pair.Token,
		"refresh": pair.RefreshToken,
	} {
		if family := claimsOf(t, token).SessionID; family != "legacy-refresh-jti" {
			t.Fatalf("%s family = %q, want legacy-refresh-jti", name, family)
		}
	}
	if revoke := postCLIRevoke(handler, "", legacy); revoke.Code != http.StatusOK {
		t.Fatalf("revoke legacy ancestor = %d: %s", revoke.Code, revoke.Body.String())
	}
	if revoked, err := denylist.IsSessionRevoked("legacy-refresh-jti"); err != nil || !revoked {
		t.Fatalf("legacy family revoke = (%v, %v)", revoked, err)
	}
}
