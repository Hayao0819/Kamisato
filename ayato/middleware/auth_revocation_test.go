package middleware

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/auth"
)

func TestRequireAdminRevokedTokenRejected(t *testing.T) {
	middleware, signer := testMiddleware(t)
	middleware.WithDenylist(fakeDenylist{revoked: map[string]bool{"revoked-jti": true}})

	revoked, err := signer.Sign(auth.Claims{
		Typ: auth.TypCLI, GitHubID: 42, Login: "alice", Name: "cli",
		JTI: "revoked-jti", Exp: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("sign revoked: %v", err)
	}
	response := run(middleware, false, func(request *http.Request) {
		request.Header.Set("Authorization", "Bearer "+revoked)
	})
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token: status = %d, want 401", response.Code)
	}

	live, err := signer.Sign(auth.Claims{
		Typ: auth.TypCLI, GitHubID: 42, Login: "alice", Name: "cli",
		JTI: "live-jti", Exp: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("sign live: %v", err)
	}
	response = run(middleware, false, func(request *http.Request) {
		request.Header.Set("Authorization", "Bearer "+live)
	})
	if response.Code != http.StatusOK {
		t.Fatalf("non-revoked token: status = %d, want 200", response.Code)
	}
}

func TestRequireAdminRevokedSessionFamilyRejected(t *testing.T) {
	middleware, signer := testMiddleware(t)
	middleware.WithDenylist(fakeDenylist{
		sessionRevoked: map[string]bool{"session-1": true},
	})
	token, err := signer.Sign(auth.Claims{
		Typ: auth.TypCLI, GitHubID: 42, Login: "alice",
		JTI: "fresh-jti", SessionID: "session-1", Exp: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	response := run(middleware, false, func(request *http.Request) {
		request.Header.Set("Authorization", "Bearer "+token)
	})
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session descendant: status = %d, want 401", response.Code)
	}
}

func TestRequireAdminCookieChecksSessionRevocation(t *testing.T) {
	middleware, signer := testMiddleware(t)
	token, err := signer.Sign(auth.Claims{
		Typ: auth.TypSession, GitHubID: 42, Login: "alice",
		SessionID: "session-1", Exp: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	request := func(request *http.Request) {
		request.AddCookie(&http.Cookie{
			Name: middleware.sessionCookieName(), Value: token,
		})
		request.Header.Set("Sec-Fetch-Site", "same-origin")
	}

	tests := []struct {
		name     string
		denylist fakeDenylist
		want     int
	}{
		{
			name:     "revoked",
			denylist: fakeDenylist{sessionRevoked: map[string]bool{"session-1": true}},
			want:     http.StatusUnauthorized,
		},
		{
			name:     "backend failure",
			denylist: fakeDenylist{err: errors.New("denylist unavailable")},
			want:     http.StatusServiceUnavailable,
		},
		{name: "live", denylist: fakeDenylist{}, want: http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			middleware.WithDenylist(test.denylist)
			if response := run(middleware, false, request); response.Code != test.want {
				t.Fatalf("status = %d, want %d", response.Code, test.want)
			}
		})
	}
}

func TestRequireAdminDenylistFailureFailsClosed(t *testing.T) {
	middleware, signer := testMiddleware(t)
	middleware.WithDenylist(fakeDenylist{err: errors.New("denylist unavailable")})
	token, err := signer.Sign(auth.Claims{
		Typ: auth.TypCLI, GitHubID: 42, Login: "alice",
		JTI: "checked", Exp: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	response := run(middleware, false, func(request *http.Request) {
		request.Header.Set("Authorization", "Bearer "+token)
	})
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("denylist failure: status = %d, want 503", response.Code)
	}
}

func TestRequireAdminExpiredAccessTokenHint(t *testing.T) {
	middleware, signer := testMiddleware(t)
	expired, err := signer.Sign(auth.Claims{
		Typ: auth.TypCLI, GitHubID: 42, Login: "alice", Name: "cli",
		JTI: "a-old", Exp: time.Now().Add(-time.Minute),
	})
	if err != nil {
		t.Fatalf("sign expired: %v", err)
	}

	response := run(middleware, false, func(request *http.Request) {
		request.Header.Set("Authorization", "Bearer "+expired)
	})
	if response.Code != http.StatusUnauthorized ||
		response.Header().Get("X-Access-Token-Expired") != "1" {
		t.Fatalf("expired access: status = %d headers = %v", response.Code, response.Header())
	}

	response = run(middleware, false, func(request *http.Request) {
		request.Header.Set("Authorization", "Bearer not-a-token")
	})
	if response.Code != http.StatusUnauthorized ||
		response.Header().Get("X-Access-Token-Expired") == "1" {
		t.Fatalf("garbage token: status = %d headers = %v", response.Code, response.Header())
	}
}

func TestRequireAdminBasicTokenBlinkyOnly(t *testing.T) {
	middleware, signer := testMiddleware(t)
	token := cliToken(t, signer, 42, "alice")
	basic := func(request *http.Request) {
		request.SetBasicAuth("anything", token)
	}

	if response := run(middleware, true, basic); response.Code != http.StatusOK {
		t.Fatalf("basic blinky token: status = %d, want 200", response.Code)
	}
	if response := run(middleware, false, basic); response.Code != http.StatusUnauthorized {
		t.Fatalf("basic admin token: status = %d, want 401", response.Code)
	}
}
