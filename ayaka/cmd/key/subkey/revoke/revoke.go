package revokecmd

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/keyinput"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/pacman/sign"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	var reason, reasonText string
	cmd := &cobra.Command{
		Use:   "revoke <fingerprint>",
		Short: "Revoke a signing subkey by fingerprint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parsed, err := sign.ParseRevocationReason(reason)
			if err != nil {
				return err
			}
			key, passphrase, err := keyinput.DefaultProvider().Load(keyinput.Read(cmd))
			if err != nil {
				return err
			}
			if err := key.RevokeSubkey(args[0], parsed, reasonText, passphrase); err != nil {
				return errors.WrapErr(err, "failed to revoke subkey")
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Revoked the subkey. Republish the keyring so the revocation reaches users.")
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "compromised", "Revocation reason: superseded|retired|compromised|unspecified")
	cmd.Flags().StringVar(&reasonText, "reason-text", "", "Free-text explanation stored in the revocation")
	return cmd
}
