package keycmd

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

func importCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "import [file]",
		Short: "Import an existing private key so ayaka can manage it",
		Long:  "Adopt an established signing key (e.g. 'gpg --export-secret-keys --armor <id>') instead of generating a new one, preserving the fingerprint users already trust. Reads the key from a file, or from stdin when the argument is omitted or '-'.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := shared.KeyDir(cmd)
			if err != nil {
				return err
			}

			var r io.Reader = cmd.InOrStdin()
			if len(args) == 1 && args[0] != "-" {
				f, err := os.Open(args[0])
				if err != nil {
					return errwrap.WrapErr(err, "open key file")
				}
				defer func() { _ = f.Close() }()
				r = f
			}

			// The passphrase both unlocks the imported key and re-encrypts it at rest.
			pass, err := shared.Passphrase(cmd, true)
			if err != nil {
				return err
			}
			k, err := sign.ImportSigningKey(dir, r, pass, force)
			if err != nil {
				return errwrap.WrapErr(err, "failed to import signing key")
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Imported signing key into %s\n", dir)
			fmt.Fprintf(out, "Primary fingerprint: %s\n", k.PrimaryFingerprint())
			if !hasUsableSigningSubkey(k) {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: no valid signing subkey; add one with 'ayaka key subkey add' before signing.")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing key in the key directory")
	return cmd
}

// hasUsableSigningSubkey reports whether the key has a signing subkey that could
// actually sign now (not revoked, not expired).
func hasUsableSigningSubkey(k *sign.SigningKey) bool {
	for _, s := range k.Subkeys() {
		if s.CanSign && !s.Revoked && (s.Expires.IsZero() || s.Expires.After(time.Now())) {
			return true
		}
	}
	return false
}
