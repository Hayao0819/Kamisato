package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/badgerkv"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

func denylistHandler(t *testing.T, adminID int64) (*Handler, *fakeDenylistRepo, *auth.Signer) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	store, err := badgerkv.New(t.TempDir())
	if err != nil {
		t.Fatalf("badgerkv.New: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	cfg := &conf.AyatoConfig{}
	cfg.Auth.PublicOrigin = "https://repo.example.com"

	authRepo := repository.NewAuthRepository(store)
	if err := authRepo.AddAdmin(adminID, "alice"); err != nil {
		t.Fatalf("AddAdmin: %v", err)
	}
	dl := &fakeDenylistRepo{}
	svc := service.New(nil, nil, authRepo, nil, cfg).WithDenylist(dl)
	signer, err := auth.NewSigner([]string{testSecret})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	return New(svc, cfg).WithAuth(signer), dl, signer
}

func jtiOf(t *testing.T, token string) string {
	return claimsOf(t, token).JTI
}

func claimsOf(t *testing.T, token string) auth.Claims {
	t.Helper()
	payloadB64, _, ok := strings.Cut(token, ".")
	if !ok {
		t.Fatalf("malformed token %q", token)
	}
	raw, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		t.Fatalf("decode token payload: %v", err)
	}
	var c auth.Claims
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("unmarshal claims: %v", err)
	}
	return c
}

// A redeemed code must be rejected on replay: the second exchange of the same code
// reads as used (400). The kv-backed guard closes the window the stateless code leaves open.
func TestCLIExchangeRejectsCodeReplay(t *testing.T) {
	h, _, signer := denylistHandler(t, 42)
	store, err := badgerkv.New(t.TempDir())
	if err != nil {
		t.Fatalf("badgerkv.New: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	h.WithReplayGuard(repository.NewReplayGuard(store))

	const verifier = "cli-verifier-cli-verifier-cli-verifier-0123"
	challenge := oauth2.S256ChallengeFromVerifier(verifier)
	code, err := signer.Sign(auth.Claims{
		Typ:       auth.TypCodeCLI,
		GitHubID:  42,
		Login:     "alice",
		Challenge: challenge,
		Exp:       time.Now().Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("sign code: %v", err)
	}

	exchange := func() int {
		r := gin.New()
		r.POST("/exchange", h.CLIExchangeHandler)
		w := httptest.NewRecorder()
		body := `{"code":"` + code + `","code_verifier":"` + verifier + `"}`
		req := httptest.NewRequest(http.MethodPost, "/exchange", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w.Code
	}

	if got := exchange(); got != http.StatusOK {
		t.Fatalf("first exchange = %d, want 200 (fresh code redeems)", got)
	}
	if got := exchange(); got != http.StatusBadRequest {
		t.Fatalf("replayed exchange = %d, want 400 (code already used)", got)
	}
}

// The web bearer exchange must mint a token carrying a jti, and that jti must be
// honoured by the denylist so MeHandler reads a revoked token as unauthenticated.
func TestWebExchangeMintsRevocableBearer(t *testing.T) {
	h, dl, _ := denylistHandler(t, 42)

	const verifier = "cli-verifier-cli-verifier-cli-verifier-0123"
	challenge := oauth2.S256ChallengeFromVerifier(verifier)
	code, err := h.signer.Sign(auth.Claims{
		Typ:       auth.TypCodeWeb,
		GitHubID:  42,
		Login:     "alice",
		Challenge: challenge,
		Exp:       time.Now().Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("sign code: %v", err)
	}

	r := gin.New()
	r.POST("/exchange", h.WebExchangeHandler)
	w := httptest.NewRecorder()
	body := `{"code":"` + code + `","code_verifier":"` + verifier + `"}`
	req := httptest.NewRequest(http.MethodPost, "/exchange", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("exchange status = %d, want 200 (body %s)", w.Code, w.Body.String())
	}
	var resp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal exchange response: %v", err)
	}
	jti := jtiOf(t, resp.Token)
	if jti == "" {
		t.Fatal("web bearer must carry a jti so it is individually revocable")
	}

	me := func() bool {
		mr := gin.New()
		mr.GET("/me", h.MeHandler)
		mw := httptest.NewRecorder()
		mreq := httptest.NewRequest(http.MethodGet, "/me", nil)
		mreq.Header.Set("Authorization", "Bearer "+resp.Token)
		mr.ServeHTTP(mw, mreq)
		var body struct {
			Authenticated bool `json:"authenticated"`
		}
		_ = json.Unmarshal(mw.Body.Bytes(), &body)
		return body.Authenticated
	}

	if !me() {
		t.Fatal("fresh web bearer must authenticate")
	}
	if err := dl.Revoke(jti, time.Hour); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if me() {
		t.Fatal("a revoked web bearer must read as unauthenticated (fail closed)")
	}
}
