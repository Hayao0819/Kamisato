package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/Hayao0819/Kamisato/ayato/router"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/gin-gonic/gin"
)

// stubChecker satisfies the middleware's (unexported) adminChecker structurally.
type stubChecker struct{ admins map[int64]bool }

func (s stubChecker) IsAdmin(id int64) bool { return s.admins[id] }

// The miko job metadata/stats reads must require auth, while /jobs/:id/logs stays
// public; this also proves SetRoute registers the moved routes without a gin
// route-collision panic. A real front server is used (not a recorder) because the
// reverse proxy needs an http.CloseNotifier ResponseWriter.
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

	// Live-log streaming stays public (EventSource cannot carry a bearer token).
	if code := get("/api/unstable/jobs/j1/logs", ""); code != http.StatusOK {
		t.Errorf("GET /jobs/:id/logs without creds = %d, want 200 (still public)", code)
	}
}
