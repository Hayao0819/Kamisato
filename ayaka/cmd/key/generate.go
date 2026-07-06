package keycmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

func generateCmd() *cobra.Command {
	var (
		name         string
		email        string
		expire       time.Duration
		subkeyExpire time.Duration
	)
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a new signing key (primary + signing subkey)",
		Long:  "Create the repository's OpenPGP identity: a primary key (the trust anchor) and a signing subkey used for package signatures. The primary fingerprint never changes as subkeys are rotated.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := shared.KeyDir(cmd)
			if err != nil {
				return err
			}
			pass, err := shared.Passphrase(cmd, true)
			if err != nil {
				return err
			}
			k, err := sign.GenerateSigningKey(dir, name, email, expire, subkeyExpire, pass)
			if err != nil {
				return errwrap.WrapErr(err, "failed to generate signing key")
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Generated signing key in %s\n", dir)
			fmt.Fprintf(out, "Primary fingerprint: %s\n", k.PrimaryFingerprint())
			fmt.Fprintln(out, "Back it up now: ayaka key export --secret --output key-backup.asc")
			fmt.Fprintln(out, "Then publish the keyring: ayaka keyring publish <repo> --name <keyring>")
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Key owner name (required)")
	cmd.Flags().StringVar(&email, "email", "", "Key owner email (required)")
	cmd.Flags().DurationVar(&expire, "expire", 0, "Primary key validity (e.g. 43800h for 5y); 0 = never")
	cmd.Flags().DurationVar(&subkeyExpire, "subkey-expire", 365*24*time.Hour, "Signing subkey validity; 0 = never")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("email")
	return cmd
}
