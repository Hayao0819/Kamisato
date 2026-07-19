package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/service"
)

func postCLIRevoke(h *AuthHandler, access, refresh string) *httptest.ResponseRecorder {
	r := gin.New()
	r.POST("/revoke", h.RevokeCLIHandler)
	w := httptest.NewRecorder()
	body := strings.NewReader("")
	if refresh != "" {
		body = strings.NewReader(`{"refresh_token":"` + refresh + `"}`)
	}
	req := httptest.NewRequest(http.MethodPost, "/revoke", body)
	if refresh != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if access != "" {
		req.Header.Set("Authorization", "Bearer "+access)
	}
	r.ServeHTTP(w, req)
	return w
}

// fakeDenylistRepo implements repository.DenylistRepository in-memory.
type fakeDenylistRepo struct {
	mu       sync.Mutex
	revoked  map[string]bool
	sessions map[string]bool
}

func (f *fakeDenylistRepo) Revoke(jti string, _ time.Duration) error {
	f.mark(&f.revoked, jti)
	return nil
}

func (f *fakeDenylistRepo) mark(target *map[string]bool, key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if *target == nil {
		*target = map[string]bool{}
	}
	(*target)[key] = true
}

func (f *fakeDenylistRepo) Consume(jti string, _ time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.revoked == nil {
		f.revoked = make(map[string]bool)
	}
	if f.revoked[jti] {
		return false, nil
	}
	f.revoked[jti] = true
	return true, nil
}

func (f *fakeDenylistRepo) IsRevoked(jti string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.revoked[jti], nil
}

func (f *fakeDenylistRepo) RevokeSession(sessionID string, _ time.Duration) error {
	f.mark(&f.sessions, sessionID)
	return nil
}

func (f *fakeDenylistRepo) IsSessionRevoked(sessionID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sessions[sessionID], nil
}

func TestRevokeCLIHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	signer, err := auth.NewSigner([]string{"0123456789abcdef0123456789abcdef"})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}

	call := func(h *AuthHandler, authz string) *httptest.ResponseRecorder {
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
	if w := call(&AuthHandler{}, ""); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured: status = %d, want 503", w.Code)
	}

	dl := &fakeDenylistRepo{}
	svc := service.New(nil, nil, nil, nil, nil).WithDenylist(dl)
	h := NewAuthHandler(svc, svc, nil).WithSigner(signer)

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
	if revoked, err := dl.IsRevoked("abc"); err != nil || !revoked {
		t.Fatal("the token's jti must be denylisted after a successful revoke")
	}
}
