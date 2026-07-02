package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/gin-gonic/gin"
)

// fakeDenylistRepo implements repository.DenylistRepository in-memory.
type fakeDenylistRepo struct{ revoked map[string]bool }

func (f *fakeDenylistRepo) Revoke(jti string, _ time.Duration) error {
	if f.revoked == nil {
		f.revoked = map[string]bool{}
	}
	f.revoked[jti] = true
	return nil
}

func (f *fakeDenylistRepo) IsRevoked(jti string) bool { return f.revoked[jti] }

func TestRevokeCLIHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	signer, err := auth.NewSigner([]string{"0123456789abcdef0123456789abcdef"})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}

	call := func(h *Handler, authz string) *httptest.ResponseRecorder {
		r := gin.New()
		r.POST("/revoke", h.RevokeCLIHandler)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/revoke", nil)
		if authz != "" {
			req.Header.Set("Authorization", authz)
		}
		r.ServeHTTP(w, req)
		return w
	}

	// Unconfigured (no signer/denylist) -> 503.
	if w := call(&Handler{}, ""); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured: status = %d, want 503", w.Code)
	}

	dl := &fakeDenylistRepo{}
	h := &Handler{signer: signer, denylist: dl}

	if w := call(h, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("no bearer: status = %d, want 401", w.Code)
	}
	if w := call(h, "Bearer not-a-token"); w.Code != http.StatusUnauthorized {
		t.Fatalf("garbage token: status = %d, want 401", w.Code)
	}

	// A CLI token with no jti cannot be individually revoked -> 409.
	noJTI, _ := signer.Sign(auth.Claims{Typ: auth.TypCLI, GitHubID: 1, Exp: time.Now().Add(time.Hour)})
	if w := call(h, "Bearer "+noJTI); w.Code != http.StatusConflict {
		t.Fatalf("token without jti: status = %d, want 409", w.Code)
	}

	// A valid CLI token with a jti is denylisted and answered 200.
	tok, _ := signer.Sign(auth.Claims{Typ: auth.TypCLI, GitHubID: 1, JTI: "abc", Exp: time.Now().Add(time.Hour)})
	if w := call(h, "Bearer "+tok); w.Code != http.StatusOK {
		t.Fatalf("valid revoke: status = %d, want 200", w.Code)
	}
	if !dl.IsRevoked("abc") {
		t.Fatal("the token's jti must be denylisted after a successful revoke")
	}
}
