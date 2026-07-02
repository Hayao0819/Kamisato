package servercmd

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/Hayao0819/Kamisato/internal/oauth"
	"github.com/spf13/cobra"
)

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
			device, err := cmd.Flags().GetBool("device")
			if err != nil {
				return err
			}

			if tokenFlag != "" {
				return saveLogin(serverURL, "token", tokenFlag, setDefault)
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer stop()

			login := func() (string, string, error) {
				// Device flow: no loopback listener, so it works over SSH/CI where the
				// browser is on another machine.
				if device {
					return oauth.DeviceLogin(ctx, serverURL, oauth.WithDeviceOutput(cmd.OutOrStdout()))
				}
				return oauth.LoopbackLogin(ctx, serverURL,
					oauth.WithNoBrowser(noBrowser),
					oauth.WithBrowserOpener(shared.OpenBrowser),
					oauth.WithOutput(cmd.OutOrStdout()),
				)
			}
			token, loginName, err := login()
			if err != nil {
				return err
			}
			if err := saveLogin(serverURL, loginName, token, setDefault); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s として %s にログインしました\n", loginName, serverURL)
			return nil
		},
	}

	cmd.Flags().Bool("default", false, "Set server as default for repo and miko commands")
	cmd.Flags().String("token", "", "Store the given CLI token directly (headless; skips the browser)")
	cmd.Flags().Bool("no-browser", false, "Print the login URL instead of opening a browser")
	cmd.Flags().Bool("device", false, "Use the device authorization flow (no local browser; for SSH/CI)")

	return cmd
}

// The single token serves both blinky Basic and Bearer paths.
func saveLogin(server, login, token string, setDefault bool) error {
	db, err := blinkyutils.ReadServerDB()
	if err != nil {
		return err
	}
	entry := db.Servers[server]
	entry.Username = login
	// Prefer the OS keyring; only when it is unavailable does the token stay in the
	// file DB. Storing to the keyring also migrates a previously file-stored token.
	if blinkyutils.StoreSecret(server, token) {
		entry.Password = ""
	} else {
		entry.Password = token
	}
	db.Servers[server] = entry
	if setDefault {
		db.DefaultServer = server
	}
	return blinkyutils.SaveServerDB(db)
}
