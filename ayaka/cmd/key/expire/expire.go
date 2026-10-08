package expirecmd

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
		expire  time.Duration
		subkeys bool
		subkey  string
	)
	cmd := &cobra.Command{
		Use:   "expire",
		Short: "Extend the validity of the primary key and/or subkeys",
		Long:  "Renew an expiring or expired key without changing its fingerprint, so users keep trusting it. By default it extends the primary; add --subkeys to also extend all signing subkeys, or --subkey <fpr> to extend one subkey only.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			k, pass, err := keyinput.DefaultProvider().Load(keyinput.Read(cmd))
			if err != nil {
				return err
			}
			targets := sign.ExpireTargets{}
			if subkey != "" {
				targets.Subkey = subkey
			} else {
				targets.Primary = true
				targets.AllSubkeys = subkeys
			}
			if err := k.SetExpiry(expire, targets, pass); err != nil {
				return errors.WrapErr(err, "failed to extend expiry")
			}
			out := cmd.OutOrStdout()
			when := "never"
			if expire > 0 {
				when = time.Now().Add(expire).Format("2006-01-02")
			}
			fmt.Fprintf(out, "Extended validity to %s. Republish the keyring so users receive the renewal.\n", when)
			return nil
		},
	}
	cmd.Flags().DurationVar(&expire, "expire", 0, "New validity from now (e.g. 17520h for 2y); 0 = never")
	cmd.Flags().BoolVar(&subkeys, "subkeys", false, "Also extend all signing subkeys together with the primary")
	cmd.Flags().StringVar(&subkey, "subkey", "", "Extend only this subkey by fingerprint (not the primary)")
	cmd.MarkFlagsMutuallyExclusive("subkey", "subkeys")
	_ = cmd.MarkFlagRequired("expire")
	return cmd
}
