package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayato/aur"
)

func aurCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aur",
		Short: "AUR catalog tooling",
	}
	cmd.AddCommand(aurKeygenCmd())
	return cmd
}

func aurKeygenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "keygen",
		Short: "Generate an Ed25519 catalog-signing seed",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			seed, err := aur.GenerateSeed()
			if err != nil {
				return err
			}
			signer, err := aur.NewCatalogSigner(seed, 0)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "AYATO_AUR_SIGNING_SEED=%s\n", seed)
			fmt.Fprintf(out, "# key_id: %s\n", signer.KeyID())
			fmt.Fprintf(out, "# pin this pubkey in kayo: %s\n", signer.PublicKeyB64())
			return nil
		},
	}
}
