package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/Hayao0819/Kamisato/ayato/router"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

// The /repo index page's CSS/JS were externalized so the strict CSP allows them.
// They must resolve same-origin (200 + right content type) and coexist with the
// /repo/:repo/:arch param routes without a route-collision panic in SetRoute.
func TestRepoIndexAssets(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &conf.AyatoConfig{}
	e := gin.New()
	h := handler.New(nil, cfg)
	m := middleware.New(cfg)
	if err := router.SetRoute(e, h, m); err != nil {
		t.Fatalf("SetRoute: %v", err)
	}

	for _, tc := range []struct{ path, wantType string }{
		{"/repo/_assets/index.css", "text/css; charset=utf-8"},
		{"/repo/_assets/index.js", "text/javascript; charset=utf-8"},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		e.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", tc.path, w.Code)
		}
		if got := w.Header().Get("Content-Type"); got != tc.wantType {
			t.Errorf("GET %s Content-Type = %q, want %q", tc.path, got, tc.wantType)
		}
		if w.Body.Len() == 0 {
			t.Errorf("GET %s returned an empty body", tc.path)
		}
	}
}
