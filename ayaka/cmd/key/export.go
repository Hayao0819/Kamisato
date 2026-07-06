package keycmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
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
			k, _, err := shared.LoadSigningKey(cmd)
			if err != nil {
				return err
			}
			var armored string
			if secret {
				armored, err = k.ExportSecretArmored()
			} else {
				armored, err = k.ExportPublicArmored()
			}
			if err != nil {
				return errwrap.WrapErr(err, "failed to export key")
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
			return errwrap.WrapErr(os.WriteFile(output, []byte(armored), perm), "failed to write key")
		},
	}
	cmd.Flags().BoolVar(&secret, "secret", false, "Export the full private key (for offline backup) instead of the public key")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Write to a file instead of stdout")
	return cmd
}
