package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/badgerkv"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

const testSecret = "0123456789abcdef0123456789abcdef" // 32 bytes

func testHandler(t *testing.T) (*AuthHandler, service.Servicer, *auth.Signer) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	store, err := badgerkv.New(t.TempDir())
	if err != nil {
		t.Fatalf("badgerkv.New: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	cfg := &conf.AyatoConfig{}
	cfg.Auth.GitHub.ClientID = "cid"
	cfg.Auth.GitHub.ClientSecret = "secret"
	cfg.Auth.PublicOrigin = "https://repo.example.com"

	// The handler reaches the allowlist through the service, so back it with a
	// real service over a badgerkv AuthRepository.
	svc := service.New(nil, nil, repository.NewAuthRepository(store), nil, cfg)
	signer, err := auth.NewSigner([]string{testSecret})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}

	h := NewAuthHandler(svc, svc, cfg).WithSigner(signer)
	return h, svc, signer
}

// Bad ports must 400, not redirect (no open-redirect surface).
func TestCLIStartRejectsBadPort(t *testing.T) {
	h, _, _ := testHandler(t)

	bad := []string{"", "abc", "12.5", "-1", "0", "70000", "8080x", "0x1f", " 8080"}
	for _, p := range bad {
		r := gin.New()
		r.GET("/cli/start", h.CLIStartHandler)
		w := httptest.NewRecorder()
		u := "/cli/start?challenge=c&state=s&port=" + url.QueryEscape(p)
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, u, nil))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("port %q: status = %d, want 400", p, w.Code)
		}
	}
}

// A good port redirects to GitHub; the state is the signed token carrying the
// port, challenge, and ayaka's state.
func TestCLIStartAcceptsGoodPort(t *testing.T) {
	h, _, signer := testHandler(t)
	r := gin.New()
	r.GET("/cli/start", h.CLIStartHandler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/cli/start?challenge=chal&state=mystate&port=49160", nil))

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "github.com/login/oauth/authorize") {
		t.Fatalf("Location = %q, want GitHub authorize redirect", loc)
	}

	// State must verify as a CLI flow with the port, challenge, and ayaka's state —
	// never a caller-supplied URL.
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	rec, err := signer.VerifyTyp(u.Query().Get("state"), auth.TypState)
	if err != nil {
		t.Fatalf("verify state: %v", err)
	}
	if !rec.CLI || rec.Port != 49160 || rec.Challenge != "chal" || rec.CLIState != "mystate" {
		t.Fatalf("state rec = %+v, want CLI/port=49160/challenge=chal/cliState=mystate", rec)
	}
}

// The loopback is built from the stored integer port only (host pinned to
// 127.0.0.1) and the redirect carries a one-time CODE, never an arbitrary target
// or a token.
func TestCLILoopbackReconstructedServerSide(t *testing.T) {
	h, _, signer := testHandler(t)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/cb?state=s", nil)

	st := &auth.Claims{Typ: auth.TypState, CLI: true, Port: 49170, Challenge: "chal", CLIState: "orig-state"}
	h.finishCLILogin(c, st, githubUser{ID: 42, Login: "alice"})

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	loc := rec.Header().Get("Location")
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("parse Location %q: %v", loc, err)
	}
	if u.Scheme != "http" || u.Host != "127.0.0.1:49170" {
		t.Fatalf("loopback = %q://%q, want http://127.0.0.1:49170", u.Scheme, u.Host)
	}
	q := u.Query()
	if q.Get("state") != "orig-state" {
		t.Fatalf("loopback state = %q, want ayaka's original %q", q.Get("state"), "orig-state")
	}
	code := q.Get("code")
	if code == "" {
		t.Fatalf("loopback redirect must carry a one-time code")
	}
	// The code must verify as a signed one-time CLI code (never a token, and never
	// a web-bearer code).
	if _, err := signer.VerifyTyp(code, auth.TypCodeCLI); err != nil {
		t.Fatalf("loopback code must be a signed one-time CLI code: %v", err)
	}
	if _, err := signer.VerifyTyp(code, auth.TypCLI); err == nil {
		t.Fatalf("loopback code must never verify as a CLI token")
	}
	if _, err := signer.VerifyTyp(code, auth.TypCodeWeb); err == nil {
		t.Fatalf("a CLI loopback code must never verify as a web-bearer code")
	}
}

// Login sets the binding cookie and signs its hash into the state, so a callback
// from a different browser cannot redeem it.
func TestGitHubLoginBindsStateToBrowser(t *testing.T) {
	h, _, signer := testHandler(t)
	r := gin.New()
	r.GET("/login", h.GitHubLoginHandler)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/login", nil))

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", w.Code)
	}
	u, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	state := u.Query().Get("state")
	if state == "" {
		t.Fatalf("no state in redirect")
	}

	var nonce string
	for _, ck := range w.Result().Cookies() {
		if ck.Name == oauthStateCookieName {
			nonce = ck.Value
			if !ck.HttpOnly || ck.SameSite != http.SameSiteLaxMode {
				t.Fatalf("binding cookie must be HttpOnly + SameSite=Lax, got %+v", ck)
			}
		}
	}
	if nonce == "" {
		t.Fatalf("login must set the %s binding cookie", oauthStateCookieName)
	}

	// State must verify as a web (non-CLI) flow carrying the cookie nonce's hash as
	// its binding.
	rec, err := signer.VerifyTyp(state, auth.TypState)
	if err != nil {
		t.Fatalf("verify state: %v", err)
	}
	if rec.CLI {
		t.Fatalf("web state must not be marked CLI")
	}
	if rec.Binding != auth.HashHex(nonce) {
		t.Fatalf("state binding = %q, want sha256(cookie) %q", rec.Binding, auth.HashHex(nonce))
	}
}
