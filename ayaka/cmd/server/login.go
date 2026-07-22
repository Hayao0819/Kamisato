package servercmd

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/auth/oauth"
	"github.com/Hayao0819/Kamisato/internal/serverstore"
)

// LoginCmd logs into ayato via a GitHub OAuth loopback (RFC 8252) + PKCE flow
// and stores the issued CLI token. --token skips the browser for headless use.
func LoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "login <server>",
		Short:             "Log in to an ayato server via GitHub OAuth",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
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
			tokenStdin, err := cmd.Flags().GetBool("token-stdin")
			if err != nil {
				return err
			}
			if tokenFlag != "" && tokenStdin {
				return fmt.Errorf("--token and --token-stdin cannot be used together")
			}
			if tokenStdin {
				scanner := bufio.NewScanner(cmd.InOrStdin())
				if scanner.Scan() {
					tokenFlag = scanner.Text()
				}
				if err := scanner.Err(); err != nil {
					return err
				}
				if tokenFlag == "" {
					return fmt.Errorf("no Bearer token was read from stdin")
				}
			}
			noBrowser, err := cmd.Flags().GetBool("no-browser")
			if err != nil {
				return err
			}
			device, err := cmd.Flags().GetBool("device")
			if err != nil {
				return err
			}

			if tokenFlag != "" {
				// A directly supplied token has no refresh companion; it lives its full
				// TTL like a pre-refresh login.
				return saveLogin(serverURL, "token", tokenFlag, "", setDefault)
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer stop()

			login := func() (string, string, string, error) {
				// Device flow: no loopback listener, so it works over SSH/CI where the
				// browser is on another machine.
				if device {
					return oauth.DeviceLogin(ctx, serverURL, oauth.WithDeviceOutput(cmd.OutOrStdout()))
				}
				return oauth.LoopbackLogin(ctx, serverURL,
					oauth.WithNoBrowser(noBrowser),
					oauth.WithBrowserOpener(oauth.OpenSystemBrowser),
					oauth.WithOutput(cmd.OutOrStdout()),
				)
			}
			token, refresh, loginName, err := login()
			if err != nil {
				return err
			}
			if err := saveLogin(serverURL, loginName, token, refresh, setDefault); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Logged in to %s as %s\n", serverURL, loginName)
			return nil
		},
	}

	cmd.Flags().Bool("default", false, "Set server as default for repo and miko commands")
	cmd.Flags().String("token", "", "Store the given CLI token directly (headless; skips the browser)")
	cmd.Flags().Bool("token-stdin", false, "Read a static Bearer token from stdin (headless; skips the browser)")
	cmd.Flags().Bool("no-browser", false, "Print the login URL instead of opening a browser")
	cmd.Flags().Bool("device", false, "Use the device authorization flow (no local browser; for SSH/CI)")

	return cmd
}

// saveLogin stores an OAuth pair or a static Bearer token.
func saveLogin(server, login, token, refresh string, setDefault bool) error {
	var saveErr error
	if refresh == "" {
		saveErr = serverstore.SaveStaticToken(server, login, token)
	} else {
		saveErr = serverstore.SaveTokens(server, login, token, refresh)
	}
	if saveErr != nil {
		return saveErr
	}
	if !setDefault {
		return nil
	}
	return serverstore.SetDefault(server)
}
