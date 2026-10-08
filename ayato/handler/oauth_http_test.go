package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type githubRoundTripFunc func(*http.Request) (*http.Response, error)

func (f githubRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestGitHubIdentityUsesInjectedHTTPForExchangeAndUser(t *testing.T) {
	var paths []string
	client := &http.Client{Transport: githubRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		paths = append(paths, request.URL.Path)
		body := `{"access_token":"github-access","token_type":"bearer"}`
		if request.URL.Path == "/user" {
			if request.Header.Get("Authorization") != "Bearer github-access" {
				t.Fatalf("GitHub user authorization = %q", request.Header.Get("Authorization"))
			}
			body = `{"id":42,"login":"alice"}`
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	h := NewAuthHandler(nil, nil, Settings{Auth: AuthSettings{GitHubClientID: "id", GitHubClientSecret: "secret", PublicOrigin: "https://ayato.example"}}, client)
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/callback", nil)
	user, ok := h.resolveGitHubUser(ctx, "code")
	if !ok || user.ID != 42 || user.Login != "alice" {
		t.Fatalf("GitHub identity = %+v, ok=%t", user, ok)
	}
	if len(paths) != 2 || paths[0] != "/login/oauth/access_token" || paths[1] != "/user" {
		t.Fatalf("GitHub requests = %v", paths)
	}
}
