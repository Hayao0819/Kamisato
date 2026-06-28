package servercmd

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	blinky_util "github.com/BrenekH/blinky/cmd/blinky/util"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/ayatoclient"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

const loginTimeout = 3 * time.Minute

// LoginCmd logs into ayato via a GitHub OAuth loopback (RFC 8252) + PKCE flow
// and stores the issued CLI token. --token skips the browser for headless use.
func LoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login <server_url>",
		Short: "Log into an ayato server via GitHub in your browser",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			serverURL := args[0]

			setDefault, err := cmd.Flags().GetBool("default")
			if err != nil {
				return err
			}
			tokenFlag, err := cmd.Flags().GetString("token")
			if err != nil {
				return err
			}
			noBrowser, err := cmd.Flags().GetBool("no-browser")
			if err != nil {
				return err
			}

			if tokenFlag != "" {
				return saveLogin(cmd, serverURL, "token", tokenFlag, setDefault)
			}

			token, login, err := browserLogin(cmd, serverURL, noBrowser)
			if err != nil {
				return err
			}
			if err := saveLogin(cmd, serverURL, login, token, setDefault); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s として %s にログインしました\n", login, serverURL)
			return nil
		},
	}

	cmd.Flags().Bool("default", false, "Set server as default for repo and miko commands")
	cmd.Flags().String("token", "", "Store the given CLI token directly (headless; skips the browser)")
	cmd.Flags().Bool("no-browser", false, "Print the login URL instead of opening a browser")

	return cmd
}

// The single token serves both blinky Basic and Bearer paths.
func saveLogin(cmd *cobra.Command, server, login, token string, setDefault bool) error {
	db, err := blinky_util.ReadServerDB()
	if err != nil {
		return utils.WrapErr(err, "failed to read server database")
	}
	entry := db.Servers[server]
	entry.Username = login
	entry.Password = token
	db.Servers[server] = entry
	if setDefault {
		db.DefaultServer = server
	}
	return utils.WrapErr(blinky_util.SaveServerDB(db), "failed to save server database")
}

// browserLogin runs the loopback OAuth+PKCE flow.
func browserLogin(cmd *cobra.Command, server string, noBrowser bool) (token, login string, err error) {
	verifier := oauth2.GenerateVerifier()
	challenge := oauth2.S256ChallengeFromVerifier(verifier)

	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return "", "", utils.WrapErr(err, "failed to generate state")
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", "", utils.WrapErr(err, "failed to open loopback listener")
	}
	port := ln.Addr().(*net.TCPAddr).Port

	startURL := fmt.Sprintf("%s/api/unstable/auth/cli/start?port=%d&challenge=%s&state=%s",
		server, port, url.QueryEscape(challenge), url.QueryEscape(state))

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != state {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "<html><body>state が一致しません。やり直してください。</body></html>")
			errCh <- utils.NewErrf("state mismatch")
			return
		}
		code := q.Get("code")
		if code == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "<html><body>code がありません。</body></html>")
			errCh <- utils.NewErrf("missing code in callback")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, "<html><body>ログインが完了しました。このタブを閉じてください。</body></html>")
		codeCh <- code
	})

	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if noBrowser {
		fmt.Fprintf(cmd.OutOrStdout(), "次の URL をブラウザで開いてください:\n%s\n", startURL)
	} else if oerr := shared.OpenBrowser(startURL); oerr != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "次の URL をブラウザで開いてください:\n%s\n", startURL)
	}

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()

	var code string
	select {
	case code = <-codeCh:
	case err = <-errCh:
		return "", "", err
	case <-ctx.Done():
		return "", "", utils.WrapErr(ctx.Err(), "login timed out or was cancelled")
	}

	token, login, _, err = ayatoclient.ExchangeCLICode(server, code, verifier)
	if err != nil {
		return "", "", utils.WrapErr(err, "failed to exchange login code")
	}
	return token, login, nil
}
