package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/handler"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/gin-gonic/gin"
)

// The aurweb NoRoute fallback must be rate-limited: past the burst, a client is
// answered 429 instead of reaching the aurweb handler.
func TestAURNoRouteRateLimited(t *testing.T) {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	m := middleware.New(&conf.AyatoConfig{})

	var served int
	srv := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		served++
		w.WriteHeader(http.StatusOK)
	})
	SetAUR(e, m, srv, handler.NewAURHandler(nil))

	var ok, limited int
	for range 60 {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/rpc?v=5&type=info&arg=x", nil)
		req.RemoteAddr = "192.0.2.10:5555"
		e.ServeHTTP(w, req)
		switch w.Code {
		case http.StatusOK:
			ok++
		case http.StatusTooManyRequests:
			limited++
		default:
			t.Fatalf("unexpected status %d", w.Code)
		}
	}

	if ok == 0 {
		t.Fatal("expected some NoRoute requests to be served")
	}
	if limited == 0 {
		t.Fatal("expected the NoRoute fallback to rate-limit past its burst")
	}
	if served != ok {
		t.Fatalf("aurweb handler served %d times but %d requests returned 200", served, ok)
	}
}
