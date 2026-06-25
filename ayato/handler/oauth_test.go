package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/kv/badgerkv"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/gin-gonic/gin"
)

const testSecret = "0123456789abcdef0123456789abcdef" // 32 bytes

func testHandler(t *testing.T) (*Handler, *auth.AllowlistRepo, *auth.Signer) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	store, err := badgerkv.New(t.TempDir())
	if err != nil {
		t.Fatalf("badgerkv.New: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	allow := auth.NewAllowlistRepo(store)
	signer, err := auth.NewSigner([]string{testSecret})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}

	cfg := &conf.AyatoConfig{}
	cfg.Auth.GitHub.ClientID = "cid"
	cfg.Auth.GitHub.ClientSecret = "secret"
	cfg.Auth.PublicOrigin = "https://repo.example.com"

	h := New(nil, cfg).WithAuth(allow, signer)
	return h, allow, signer
}

// TestCLIStartRejectsBadPort ensures /cli/start rejects non-integer / garbage /
// out-of-range ports with 400 (no redirect, no open-redirect surface).
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

// TestCLIStartAcceptsGoodPort ensures a plain integer port is accepted and the
// flow redirects to GitHub's authorize endpoint. The state sent to GitHub is the
// signed state token carrying the integer port, challenge, and ayaka's state.
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

	// The signed state must verify as a CLI flow carrying the integer port,
	// challenge, and ayaka's original state — never a caller-supplied URL.
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

// TestCLILoopbackReconstructedServerSide verifies the callback's CLI branch
// builds the loopback URL from the stored integer port only (host pinned to
// 127.0.0.1), echoes ayaka's original state, and redirects a one-time CODE —
// never honoring an arbitrary target and never carrying a token.
func TestCLILoopbackReconstructedServerSide(t *testing.T) {
	h, _, signer := testHandler(t)

	// Drive finishCLILogin directly with state claims carrying the port and the
	// original ayaka state.
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
	// The code must verify as a signed one-time code (never a token).
	if _, err := signer.VerifyTyp(code, auth.TypCode); err != nil {
		t.Fatalf("loopback code must be a signed one-time code: %v", err)
	}
	if _, err := signer.VerifyTyp(code, auth.TypCLI); err == nil {
		t.Fatalf("loopback code must never verify as a CLI token")
	}
}

// TestGitHubLoginBindsStateToBrowser verifies the web login flow sets the
// ayato_oauth_state cookie and signs its hash into the state binding, so a
// callback from a different browser (no matching cookie) cannot redeem it.
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

	// The signed state must verify, be a web (non-CLI) flow, and carry the hash
	// of the browser-held cookie nonce as its binding.
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
