package router_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/badgerkv"
	"github.com/Hayao0819/Kamisato/ayato/router"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

// stubChecker satisfies the middleware's (unexported) adminChecker structurally.
type stubChecker struct{ admins map[int64]bool }

func (s stubChecker) IsAdmin(id int64) bool { return s.admins[id] }

// The miko job metadata/stats reads must require auth, and /jobs/:id/logs now
// requires a one-time token or an admin credential (no longer public); this also
// proves SetRoute registers the moved routes without a gin route-collision panic.
// A real front server is used (not a recorder) because the reverse proxy needs an
// http.CloseNotifier ResponseWriter.
func TestMikoJobReadsRequireAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("upstream-ok"))
	}))
	defer upstream.Close()

	cfg := &conf.AyatoConfig{}
	cfg.Miko.URL = upstream.URL
	signer, err := auth.NewSigner([]string{"0123456789abcdef0123456789abcdef"})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}

	e := gin.New()
	h := handler.New(nil, cfg).WithAuth(signer)
	m := middleware.New(cfg).WithAuth(stubChecker{admins: map[int64]bool{42: true}}, signer)
	if err := router.SetRoute(e, h, m); err != nil {
		t.Fatalf("SetRoute: %v", err)
	}
	front := httptest.NewServer(e)
	defer front.Close()

	get := func(path, authz string) int {
		req, _ := http.NewRequest(http.MethodGet, front.URL+path, nil)
		if authz != "" {
			req.Header.Set("Authorization", authz)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}

	admin, _ := signer.Sign(auth.Claims{Typ: auth.TypCLI, GitHubID: 42, Exp: time.Now().Add(time.Hour)})

	for _, path := range []string{"/api/unstable/jobs", "/api/unstable/stats", "/api/unstable/jobs/j1/artifacts"} {
		if code := get(path, ""); code != http.StatusUnauthorized {
			t.Errorf("GET %s without creds = %d, want 401", path, code)
		}
		if code := get(path, "Bearer "+admin); code != http.StatusOK {
			t.Errorf("GET %s with admin token = %d, want 200 (proxied)", path, code)
		}
	}

	// Live-log streaming is no longer public: without a one-time token or an admin
	// credential it is rejected, while an admin bearer still streams (kept working).
	if code := get("/api/unstable/jobs/j1/logs", ""); code != http.StatusUnauthorized {
		t.Errorf("GET /jobs/:id/logs without creds = %d, want 401", code)
	}
	if code := get("/api/unstable/jobs/j1/logs", "Bearer "+admin); code != http.StatusOK {
		t.Errorf("GET /jobs/:id/logs with admin token = %d, want 200 (bearer path kept)", code)
	}
}

// The SSE one-time token lets a browser open the build-log stream without a
// long-lived bearer: an admin mints a token bound to the job, streaming with it
// succeeds once, a second use is rejected (spent), and a token minted for another
// job is rejected against this one.
func TestJobLogsOneTimeToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("log-line"))
	}))
	defer upstream.Close()

	cfg := &conf.AyatoConfig{}
	cfg.Miko.URL = upstream.URL
	signer, err := auth.NewSigner([]string{"0123456789abcdef0123456789abcdef"})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}

	store, err := badgerkv.New(t.TempDir())
	if err != nil {
		t.Fatalf("badgerkv.New: %v", err)
	}
	defer func() { _ = store.Close() }()
	logTokens := repository.NewLogTokenRepository(store)

	e := gin.New()
	h := handler.New(nil, cfg).WithAuth(signer).WithLogTokens(logTokens)
	m := middleware.New(cfg).WithAuth(stubChecker{admins: map[int64]bool{42: true}}, signer).WithLogTokens(logTokens)
	if err := router.SetRoute(e, h, m); err != nil {
		t.Fatalf("SetRoute: %v", err)
	}
	front := httptest.NewServer(e)
	defer front.Close()

	admin, _ := signer.Sign(auth.Claims{Typ: auth.TypCLI, GitHubID: 42, Exp: time.Now().Add(time.Hour)})

	mint := func(job string) string {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, front.URL+"/api/unstable/jobs/"+job+"/logs/token", nil)
		req.Header.Set("Authorization", "Bearer "+admin)
		resp, rerr := http.DefaultClient.Do(req)
		if rerr != nil {
			t.Fatalf("mint: %v", rerr)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("mint status = %d, want 200", resp.StatusCode)
		}
		var body struct {
			Token string `json:"token"`
		}
		if derr := json.NewDecoder(resp.Body).Decode(&body); derr != nil || body.Token == "" {
			t.Fatalf("mint decode: %v (token %q)", derr, body.Token)
		}
		return body.Token
	}

	stream := func(job, token string) (int, string) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, front.URL+"/api/unstable/jobs/"+job+"/logs?token="+token, nil)
		resp, rerr := http.DefaultClient.Do(req)
		if rerr != nil {
			t.Fatalf("stream: %v", rerr)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}

	// A minted token streams once and returns the proxied log body.
	tok := mint("j1")
	if code, body := stream("j1", tok); code != http.StatusOK || body != "log-line" {
		t.Fatalf("first stream = %d %q, want 200 \"log-line\"", code, body)
	}
	// The same token is spent: a second use is rejected.
	if code, _ := stream("j1", tok); code != http.StatusUnauthorized {
		t.Fatalf("replayed token = %d, want 401 (spent)", code)
	}
	// A token minted for j1 must not open j2's stream.
	other := mint("j1")
	if code, _ := stream("j2", other); code != http.StatusUnauthorized {
		t.Fatalf("wrong-job token = %d, want 401", code)
	}
}
