package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/internal/conf"
)

const (
	testSecret  = "0123456789abcdef0123456789abcdef"
	testAdminID = int64(42)
)

type fakeChecker struct {
	allowed map[int64]bool
}

func (checker fakeChecker) IsAdmin(id int64) bool {
	return id > 0 && checker.allowed[id]
}

type fakeDenylist struct {
	revoked        map[string]bool
	sessionRevoked map[string]bool
	err            error
}

func (denylist fakeDenylist) IsRevoked(jti string) (bool, error) {
	return denylist.revoked[jti], denylist.err
}

func (denylist fakeDenylist) IsSessionRevoked(sessionID string) (bool, error) {
	return denylist.sessionRevoked[sessionID], denylist.err
}

func testMiddleware(t *testing.T) (*Middleware, *auth.Signer) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	checker := fakeChecker{allowed: map[int64]bool{testAdminID: true}}
	signer, err := auth.NewSigner([]string{testSecret})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	middleware := New(&conf.AyatoConfig{}).WithAuth(checker, signer)
	return middleware, signer
}

func sessionToken(t *testing.T, signer *auth.Signer, id int64, login string) string {
	t.Helper()
	token, err := signer.Sign(auth.Claims{
		Typ: auth.TypSession, GitHubID: id, Login: login,
		Exp: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("sign session: %v", err)
	}
	return token
}

func cliToken(t *testing.T, signer *auth.Signer, id int64, login string) string {
	t.Helper()
	token, err := signer.Sign(auth.Claims{
		Typ: auth.TypCLI, GitHubID: id, Login: login, Name: "cli",
		Exp: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("sign cli: %v", err)
	}
	return token
}

func run(
	middleware *Middleware,
	allowBasic bool,
	mutate func(*http.Request),
) *httptest.ResponseRecorder {
	router := gin.New()
	router.GET("/p", middleware.requireAdmin(allowBasic), func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
	})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/p", nil)
	mutate(request)
	router.ServeHTTP(response, request)
	return response
}

func TestRequireAdminFailsClosedWithoutAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	middleware := New(&conf.AyatoConfig{})
	for _, allowBasic := range []bool{true, false} {
		response := run(middleware, allowBasic, func(*http.Request) {})
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("allowBasic=%v: status = %d, want 503", allowBasic, response.Code)
		}
	}
}

func TestRequireAdminNoCredentials(t *testing.T) {
	middleware, _ := testMiddleware(t)
	if response := run(middleware, false, func(*http.Request) {}); response.Code != http.StatusUnauthorized {
		t.Fatalf("no credentials: status = %d, want 401", response.Code)
	}
}

func TestRequireAdminBearerToken(t *testing.T) {
	tests := []struct {
		name      string
		tokenType string
		gitHubID  int64
		want      int
	}{
		{name: "allowlisted CLI", tokenType: auth.TypCLI, gitHubID: 42, want: http.StatusOK},
		{name: "not allowlisted", tokenType: auth.TypCLI, gitHubID: 99, want: http.StatusForbidden},
		{name: "session type rejected", tokenType: auth.TypSession, gitHubID: 42, want: http.StatusUnauthorized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			middleware, signer := testMiddleware(t)
			token, err := signer.Sign(auth.Claims{
				Typ: test.tokenType, GitHubID: test.gitHubID,
				Login: "alice", Exp: time.Now().Add(time.Hour),
			})
			if err != nil {
				t.Fatal(err)
			}
			response := run(middleware, false, func(request *http.Request) {
				request.Header.Set("Authorization", "Bearer "+token)
			})
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d", response.Code, test.want)
			}
		})
	}
}

func TestRequireAdminCookieRequiresSecFetch(t *testing.T) {
	middleware, signer := testMiddleware(t)
	token := sessionToken(t, signer, 42, "alice")
	request := func(site string) *httptest.ResponseRecorder {
		return run(middleware, false, func(request *http.Request) {
			request.AddCookie(&http.Cookie{
				Name: middleware.cfg.Auth.CookieName(), Value: token,
			})
			if site != "" {
				request.Header.Set("Sec-Fetch-Site", site)
			}
		})
	}
	if response := request(""); response.Code != http.StatusForbidden {
		t.Fatalf("cookie without Sec-Fetch-Site: status = %d, want 403", response.Code)
	}
	if response := request("same-origin"); response.Code != http.StatusOK {
		t.Fatalf("same-origin cookie: status = %d, want 200", response.Code)
	}
}

func TestRequireAdminCookieOriginFallback(t *testing.T) {
	middleware, signer := testMiddleware(t)
	middleware.cfg.Auth.PublicOrigin = "https://repo.example.com"
	token := sessionToken(t, signer, 42, "alice")
	withCookie := func(mutate func(*http.Request)) *httptest.ResponseRecorder {
		return run(middleware, false, func(request *http.Request) {
			request.AddCookie(&http.Cookie{
				Name: middleware.cfg.Auth.CookieName(), Value: token,
			})
			mutate(request)
		})
	}

	tests := []struct {
		name   string
		header string
		value  string
		want   int
	}{
		{"matching origin", "Origin", "https://repo.example.com", http.StatusOK},
		{"foreign origin", "Origin", "https://evil.example.com", http.StatusForbidden},
		{"matching referer", "Referer", "https://repo.example.com/packages", http.StatusOK},
		{"missing origin", "", "", http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := withCookie(func(request *http.Request) {
				if test.header != "" {
					request.Header.Set(test.header, test.value)
				}
			})
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d", response.Code, test.want)
			}
		})
	}
}
