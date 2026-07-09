package ayatocmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/kayo/ayatosrc"
	"github.com/Hayao0819/Kamisato/kayo/cmd/shared"
)

func ayatoPinCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pin <name>",
		Short: "Fetch a source's advertised signing key to pin in config",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := shared.LoadConfig(cmd)
			if err != nil {
				return err
			}
			var src *conf.AyatoSource
			for i := range cfg.Ayato {
				if cfg.Ayato[i].Name == args[0] {
					src = &cfg.Ayato[i]
					break
				}
			}
			if src == nil {
				return errors.NewErrf("no ayato source named %q in config", args[0])
			}

			pub, keyID, err := ayatosrc.FetchPubkey(cmd.Context(), src.URL)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "# verify this key_id out of band before trusting it: %s\n", keyID)
			fmt.Fprintf(out, "# then set it on the %q source in config:\n", src.Name)
			fmt.Fprintf(out, "pubkey = %q\n", pub)
			return nil
		},
	}
}
