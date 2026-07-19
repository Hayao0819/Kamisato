package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeLogTokenConsumer struct {
	jobs map[string]string
	err  error
}

func (f fakeLogTokenConsumer) ConsumeLogToken(token string) (string, bool, error) {
	if f.err != nil {
		return "", false, f.err
	}
	jobID, ok := f.jobs[token]
	return jobID, ok, nil
}

func runLogAccess(
	m *Middleware,
	target string,
	mutate func(*http.Request),
) *httptest.ResponseRecorder {
	router := gin.New()
	router.GET("/logs/:id", m.RequireLogAccess(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	mutate(request)
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestRequireLogAccessOneTimeToken(t *testing.T) {
	m, _ := testMiddleware(t, 42)
	m.WithLogTokens(fakeLogTokenConsumer{jobs: map[string]string{"valid": "job-1"}})

	if response := runLogAccess(m, "/logs/job-1?token=valid", func(*http.Request) {}); response.Code != http.StatusOK {
		t.Fatalf("matching token: status = %d, want 200", response.Code)
	}
	if response := runLogAccess(m, "/logs/job-2?token=valid", func(*http.Request) {}); response.Code != http.StatusUnauthorized {
		t.Fatalf("wrong job: status = %d, want 401", response.Code)
	}
	if response := runLogAccess(m, "/logs/job-1", func(request *http.Request) {
		request.Header.Set(logTokenHeader, "valid")
	}); response.Code != http.StatusOK {
		t.Fatalf("header token: status = %d, want 200", response.Code)
	}
}

func TestRequireLogAccessTokenFailureIsFinal(t *testing.T) {
	m, signer := testMiddleware(t, 42)
	token := cliToken(t, signer, 42, "alice")
	m.WithLogTokens(fakeLogTokenConsumer{jobs: map[string]string{}})

	response := runLogAccess(m, "/logs/job-1?token=invalid", func(request *http.Request) {
		request.Header.Set("Authorization", "Bearer "+token)
	})
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("invalid one-time token with valid bearer: status = %d, want 401", response.Code)
	}

	m.WithLogTokens(fakeLogTokenConsumer{err: errors.New("token store unavailable")})
	response = runLogAccess(m, "/logs/job-1?token=valid", func(*http.Request) {})
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("token store failure: status = %d, want 503", response.Code)
	}
}

func TestRequireLogAccessFallsBackToAdmin(t *testing.T) {
	m, signer := testMiddleware(t, 42)
	token := cliToken(t, signer, 42, "alice")
	response := runLogAccess(m, "/logs/job-1", func(request *http.Request) {
		request.Header.Set("Authorization", "Bearer "+token)
	})
	if response.Code != http.StatusOK {
		t.Fatalf("admin bearer fallback: status = %d, want 200", response.Code)
	}
}
