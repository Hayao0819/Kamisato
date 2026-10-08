package revokecmd

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/keyinput"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/pacman/sign"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	var (
		reason     string
		reasonText string
		yes        bool
	)
	cmd := &cobra.Command{
		Use:   "revoke",
		Short: "Revoke the whole signing key (primary and all subkeys)",
		Long:  "Revoke the entire key. This is drastic: rebuild and republish the keyring so users pick up the revocation, and issue a new key. Use reason compromised only for an actual leak (it invalidates past signatures); superseded keeps them valid.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			parsed, err := sign.ParseRevocationReason(reason)
			if err != nil {
				return err
			}
			if !yes {
				return errors.NewErr("refusing to revoke the primary key without --yes")
			}
			k, pass, err := keyinput.DefaultProvider().Load(keyinput.Read(cmd))
			if err != nil {
				return err
			}
			if err := k.RevokePrimary(parsed, reasonText, pass); err != nil {
				return errors.WrapErr(err, "failed to revoke primary key")
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Revoked primary key %s (%s)\n", k.PrimaryFingerprint(), reason)
			if sign.IsHardRevocation(parsed) {
				fmt.Fprintln(out, "This is a hard revocation: packages signed by this key should be re-signed with a new key.")
			}
			fmt.Fprintln(out, "Next: generate a new key and run 'ayaka keyring publish' so users receive the revocation.")
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "compromised", "Revocation reason: superseded|retired|compromised|unspecified")
	cmd.Flags().StringVar(&reasonText, "reason-text", "", "Free-text explanation stored in the revocation")
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm revoking the whole key")
	return cmd
}
