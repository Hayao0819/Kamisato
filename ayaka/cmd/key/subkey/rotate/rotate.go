package rotatecmd

import (
	"fmt"
	"time"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/keyinput"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/pacman/sign"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
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
			key, passphrase, err := keyinput.DefaultProvider().Load(keyinput.Read(cmd))
			if err != nil {
				return err
			}
			if err := key.RotateSubkey(parsed, reasonText, expire, passphrase); err != nil {
				return errors.WrapErr(err, "failed to rotate subkey")
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
