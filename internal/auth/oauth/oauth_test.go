package oauth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"golang.org/x/oauth2"
)

func TestNewPKCE(t *testing.T) {
	a, err := newPKCE()
	if err != nil {
		t.Fatal(err)
	}
	if a.verifier == "" || a.state == "" {
		t.Fatalf("empty verifier/state: %+v", a)
	}
	if got := oauth2.S256ChallengeFromVerifier(a.verifier); got != a.challenge {
		t.Fatalf("challenge %q is not the S256 of the verifier %q", a.challenge, got)
	}
	b, err := newPKCE()
	if err != nil {
		t.Fatal(err)
	}
	if a.verifier == b.verifier || a.state == b.state {
		t.Fatal("two calls produced identical verifier/state; must be random per login")
	}
}

func TestCallbackHandler(t *testing.T) {
	const state = "the-state"

	t.Run("valid code", func(t *testing.T) {
		codeCh := make(chan string, 1)
		errCh := make(chan error, 1)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/?state="+state+"&code=abc", nil)
		callbackHandler(state, codeCh, errCh)(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		select {
		case got := <-codeCh:
			if got != "abc" {
				t.Fatalf("code = %q, want abc", got)
			}
		default:
			t.Fatal("no code delivered")
		}
	})

	t.Run("state mismatch", func(t *testing.T) {
		codeCh := make(chan string, 1)
		errCh := make(chan error, 1)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/?state=wrong&code=abc", nil)
		callbackHandler(state, codeCh, errCh)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
		if len(errCh) != 1 {
			t.Fatal("expected an error on the channel")
		}
		if len(codeCh) != 0 {
			t.Fatal("a mismatched state must not deliver a code")
		}
	})

	t.Run("missing code", func(t *testing.T) {
		codeCh := make(chan string, 1)
		errCh := make(chan error, 1)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/?state="+state, nil)
		callbackHandler(state, codeCh, errCh)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
		if len(errCh) != 1 {
			t.Fatal("expected an error on the channel")
		}
	})
}

// TestLoopbackLogin drives the whole flow with an injected browser opener that
// plays the redirect back into the loopback listener, and an injected exchanger,
// so no real browser or server is involved.
func TestLoopbackLogin(t *testing.T) {
	opener := func(rawURL string) error {
		u, err := url.Parse(rawURL)
		if err != nil {
			return err
		}
		q := u.Query()
		cb := fmt.Sprintf("http://127.0.0.1:%s/?state=%s&code=the-code",
			q.Get("port"), url.QueryEscape(q.Get("state")))
		resp, err := http.Get(cb)
		if err != nil {
			return err
		}
		return resp.Body.Close()
	}
	exchange := func(_ context.Context, serverURL, code, verifier string) (string, string, string, error) {
		if code != "the-code" {
			return "", "", "", fmt.Errorf("unexpected code %q", code)
		}
		if verifier == "" {
			return "", "", "", fmt.Errorf("empty verifier")
		}
		return "the-token", "the-refresh", "octocat", nil
	}

	token, refresh, login, err := LoopbackLogin(context.Background(), "https://ayato.example",
		WithBrowserOpener(opener),
		WithExchanger(exchange),
		WithOutput(io.Discard),
	)
	if err != nil {
		t.Fatal(err)
	}
	if token != "the-token" || refresh != "the-refresh" || login != "octocat" {
		t.Fatalf("got token=%q refresh=%q login=%q", token, refresh, login)
	}
}
