// Package oauth runs the ayato CLI login: an RFC 8252 loopback redirect carrying
// a PKCE challenge, whose callback code is exchanged for a CLI token. The browser
// opener and the code exchange are injectable so the flow is testable without a
// real browser or a live server.
package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/Hayao0819/Kamisato/internal/ayatoclient"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"golang.org/x/oauth2"
)

const defaultTimeout = 3 * time.Minute

// Exchanger trades the one-time callback code plus its PKCE verifier for a CLI
// token and the authenticated login name.
type Exchanger func(ctx context.Context, serverURL, code, verifier string) (token, login string, err error)

// BrowserOpener opens rawURL in the user's browser. A non-nil error tells
// LoopbackLogin to fall back to printing the URL.
type BrowserOpener func(rawURL string) error

type options struct {
	noBrowser bool
	openURL   BrowserOpener
	exchange  Exchanger
	out       io.Writer
	timeout   time.Duration
}

// Option configures LoopbackLogin.
type Option func(*options)

// WithNoBrowser prints the login URL instead of opening a browser.
func WithNoBrowser(v bool) Option { return func(o *options) { o.noBrowser = v } }

// WithBrowserOpener injects the browser launcher. With none set the URL is
// printed for the user to open manually.
func WithBrowserOpener(fn BrowserOpener) Option { return func(o *options) { o.openURL = fn } }

// WithExchanger overrides the code-for-token exchange; the default hits ayato's
// direct exchange endpoint. Chiefly a test seam.
func WithExchanger(fn Exchanger) Option { return func(o *options) { o.exchange = fn } }

// WithOutput sets where user-facing prompts are written (default os.Stdout).
func WithOutput(w io.Writer) Option { return func(o *options) { o.out = w } }

// WithTimeout bounds the wait for the browser callback (default 3m).
func WithTimeout(d time.Duration) Option { return func(o *options) { o.timeout = d } }

func defaultExchange(ctx context.Context, serverURL, code, verifier string) (string, string, error) {
	token, login, _, err := ayatoclient.ExchangeCLICode(ctx, serverURL, code, verifier)
	return token, login, err
}

type pkce struct {
	verifier  string
	challenge string
	state     string
}

func newPKCE() (pkce, error) {
	verifier := oauth2.GenerateVerifier()
	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return pkce{}, errwrap.WrapErr(err, "failed to generate state")
	}
	return pkce{
		verifier:  verifier,
		challenge: oauth2.S256ChallengeFromVerifier(verifier),
		state:     base64.RawURLEncoding.EncodeToString(stateBytes),
	}, nil
}

// callbackHandler serves the browser redirect: it validates state and forwards
// the code (or an error) over the channels, which must be buffered.
func callbackHandler(state string, codeCh chan<- string, errCh chan<- error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != state {
			writeHTML(w, http.StatusBadRequest, "state が一致しません。やり直してください。")
			errCh <- errwrap.NewErrf("state mismatch")
			return
		}
		code := q.Get("code")
		if code == "" {
			writeHTML(w, http.StatusBadRequest, "code がありません。")
			errCh <- errwrap.NewErrf("missing code in callback")
			return
		}
		writeHTML(w, http.StatusOK, "ログインが完了しました。このタブを閉じてください。")
		codeCh <- code
	}
}

func writeHTML(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, "<html><body>%s</body></html>", body)
}

// LoopbackLogin runs the loopback + PKCE login against serverURL and returns the
// issued CLI token and the authenticated login name. It honors ctx and applies
// its own timeout on top.
func LoopbackLogin(ctx context.Context, serverURL string, opts ...Option) (token, login string, err error) {
	o := options{exchange: defaultExchange, out: os.Stdout, timeout: defaultTimeout}
	for _, apply := range opts {
		apply(&o)
	}

	p, err := newPKCE()
	if err != nil {
		return "", "", err
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", "", errwrap.WrapErr(err, "failed to open loopback listener")
	}
	port := ln.Addr().(*net.TCPAddr).Port

	startURL := fmt.Sprintf("%s/api/unstable/auth/cli/start?port=%d&challenge=%s&state=%s",
		serverURL, port, url.QueryEscape(p.challenge), url.QueryEscape(p.state))

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/", callbackHandler(p.state, codeCh, errCh))

	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if o.noBrowser || o.openURL == nil {
		fmt.Fprintf(o.out, "次の URL をブラウザで開いてください:\n%s\n", startURL)
	} else if oerr := o.openURL(startURL); oerr != nil {
		fmt.Fprintf(o.out, "次の URL をブラウザで開いてください:\n%s\n", startURL)
	}

	ctx, cancel := context.WithTimeout(ctx, o.timeout)
	defer cancel()

	var code string
	select {
	case code = <-codeCh:
	case err = <-errCh:
		return "", "", err
	case <-ctx.Done():
		return "", "", errwrap.WrapErr(ctx.Err(), "login timed out or was cancelled")
	}

	token, login, err = o.exchange(ctx, serverURL, code, p.verifier)
	if err != nil {
		return "", "", errwrap.WrapErr(err, "failed to exchange login code")
	}
	return token, login, nil
}
