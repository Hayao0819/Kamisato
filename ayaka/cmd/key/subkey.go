package keycmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

func subkeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subkey",
		Short: "Manage signing subkeys",
		Long:  "Add, revoke, or rotate the signing subkeys bound to the primary key. Rotating a subkey never changes the primary fingerprint, so downstream trust is preserved.",
	}
	cmd.AddCommand(subkeyAddCmd(), subkeyRevokeCmd(), subkeyRotateCmd())
	return cmd
}

func subkeyAddCmd() *cobra.Command {
	var expire time.Duration
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new signing subkey",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			k, pass, err := loadForMutation(cmd)
			if err != nil {
				return err
			}
			if err := k.AddSubkey(expire, pass); err != nil {
				return errwrap.WrapErr(err, "failed to add subkey")
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Added a new signing subkey. Republish the keyring so it reaches users.")
			return nil
		},
	}
	cmd.Flags().DurationVar(&expire, "expire", 365*24*time.Hour, "Subkey validity; 0 = never")
	return cmd
}

func subkeyRevokeCmd() *cobra.Command {
	var (
		reason     string
		reasonText string
	)
	cmd := &cobra.Command{
		Use:   "revoke <fingerprint>",
		Short: "Revoke a signing subkey by fingerprint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parsed, err := sign.ParseRevocationReason(reason)
			if err != nil {
				return err
			}
			k, pass, err := loadForMutation(cmd)
			if err != nil {
				return err
			}
			if err := k.RevokeSubkey(args[0], parsed, reasonText, pass); err != nil {
				return errwrap.WrapErr(err, "failed to revoke subkey")
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Revoked the subkey. Republish the keyring so the revocation reaches users.")
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "compromised", "Revocation reason: superseded|retired|compromised|unspecified")
	cmd.Flags().StringVar(&reasonText, "reason-text", "", "Free-text explanation stored in the revocation")
	return cmd
}

func subkeyRotateCmd() *cobra.Command {
	var (
		reason     string
		reasonText string
		expire     time.Duration
	)
	cmd := &cobra.Command{
		Use:   "rotate",
		Short: "Revoke the current signing subkey(s) and bind a fresh one",
		Long:  "Routine rotation: revoke every active signing subkey and add a new one. Use the default reason superseded so packages signed by the old subkey remain valid.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			parsed, err := sign.ParseRevocationReason(reason)
			if err != nil {
				return err
			}
			k, pass, err := loadForMutation(cmd)
			if err != nil {
				return err
			}
			if err := k.RotateSubkey(parsed, reasonText, expire, pass); err != nil {
				return errwrap.WrapErr(err, "failed to rotate subkey")
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "Rotated the signing subkey; the primary fingerprint is unchanged.")
			fmt.Fprintln(out, "Republish the keyring so users receive the new subkey.")
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "superseded", "Revocation reason for the old subkey: superseded|retired|compromised|unspecified")
	cmd.Flags().StringVar(&reasonText, "reason-text", "", "Free-text explanation stored in the revocation")
	cmd.Flags().DurationVar(&expire, "expire", 365*24*time.Hour, "New subkey validity; 0 = never")
	return cmd
}

// loadForMutation loads the key plus the passphrase that unlocked it, so an
// operation that re-saves the key re-encrypts it with the same passphrase.
func loadForMutation(cmd *cobra.Command) (*sign.SigningKey, string, error) {
	return shared.LoadSigningKey(cmd)
}
