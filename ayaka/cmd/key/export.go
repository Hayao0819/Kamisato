package keycmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/atomicfile"
)

func exportCmd() *cobra.Command {
	var (
		secret bool
		output string
	)
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export the public key, or the full secret key with --secret",
		Long:  "Write the armored public key (default) for keyring distribution, or the full private key with --secret for offline backup. Handle a secret export as sensitive material.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			k, pass, err := shared.LoadSigningKey(cmd)
			if err != nil {
				return err
			}
			var armored string
			if secret {
				if pass == "" {
					fmt.Fprintln(cmd.ErrOrStderr(), "warning: the key has no passphrase, so this secret export is unencrypted; store it securely.")
				}
				armored, err = k.ExportSecretArmored(pass)
			} else {
				armored, err = k.ExportPublicArmored()
			}
			if err != nil {
				return errors.WrapErr(err, "failed to export key")
			}
			if output == "" || output == "-" {
				_, err = fmt.Fprint(cmd.OutOrStdout(), armored)
				return err
			}
			// A secret export is written 0600 so the private key is not world-readable.
			perm := os.FileMode(0o644)
			if secret {
				perm = 0o600
			}
			return errors.WrapErr(atomicfile.WriteFile(output, []byte(armored), perm), "failed to write key")
		},
	}
	cmd.Flags().BoolVar(&secret, "secret", false, "Export the full private key (for offline backup) instead of the public key")
	cmd.Flags().StringVar(&output, "output", "", "Write to a file instead of stdout")
	return cmd
}
