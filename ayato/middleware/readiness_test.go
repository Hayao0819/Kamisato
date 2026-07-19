package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type readinessStub bool

func (ready readinessStub) Ready() bool { return bool(ready) }

func TestRejectMutationsWhenNotReady(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	for _, test := range []struct {
		name   string
		method string
		ready  readinessStub
		status int
		called bool
	}{
		{name: "ready mutation", method: http.MethodPost, ready: true, status: http.StatusNoContent, called: true},
		{name: "draining mutation", method: http.MethodPost, ready: false, status: http.StatusServiceUnavailable},
		{name: "draining read", method: http.MethodGet, ready: false, status: http.StatusNoContent, called: true},
		{name: "draining probe", method: http.MethodHead, ready: false, status: http.StatusNoContent, called: true},
		{name: "draining preflight", method: http.MethodOptions, ready: false, status: http.StatusNoContent, called: true},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			called := false
			engine := gin.New()
			engine.Use((&Middleware{}).RejectMutationsWhenNotReady(test.ready))
			engine.Handle(test.method, "/", func(ctx *gin.Context) {
				called = true
				ctx.Status(http.StatusNoContent)
			})

			response := httptest.NewRecorder()
			engine.ServeHTTP(response, httptest.NewRequest(test.method, "/", nil))
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
			if called != test.called {
				t.Fatalf("handler called = %v, want %v", called, test.called)
			}
		})
	}
}
