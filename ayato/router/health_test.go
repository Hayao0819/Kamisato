package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type readinessStub bool

func (ready readinessStub) Ready() bool { return bool(ready) }

func TestHealthRoutesSeparateLivenessFromReadiness(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	for _, test := range []struct {
		name        string
		ready       readinessStub
		readyStatus int
	}{
		{name: "ready", ready: true, readyStatus: http.StatusOK},
		{name: "draining", ready: false, readyStatus: http.StatusServiceUnavailable},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			engine := gin.New()
			setHealthRoutes(engine, test.ready)

			health := httptest.NewRecorder()
			engine.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/health", nil))
			if health.Code != http.StatusOK || health.Body.String() != "ok" {
				t.Fatalf("/health = %d %q, want 200 ok", health.Code, health.Body.String())
			}

			ready := httptest.NewRecorder()
			engine.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/ready", nil))
			if ready.Code != test.readyStatus {
				t.Fatalf("/ready status = %d, want %d", ready.Code, test.readyStatus)
			}
			var body struct {
				Ready bool `json:"ready"`
			}
			if err := json.Unmarshal(ready.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Ready != bool(test.ready) {
				t.Fatalf("/ready body = %+v, want ready=%v", body, test.ready)
			}
		})
	}
}

func TestHealthRoutesDefaultToReady(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	setHealthRoutes(engine, nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("/ready status = %d, want 200", response.Code)
	}
}
