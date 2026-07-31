package signercmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/cliutil"
	"github.com/Hayao0819/Kamisato/miko/app"
	mikoconfig "github.com/Hayao0819/Kamisato/miko/config"
)

// Cmd runs the dedicated signer tier: it holds the host signing key and
// signs the packages build workers POST to it, so those workers can run keyless.
func Cmd() *cobra.Command {
	return &cobra.Command{
		Use:   "signer",
		Short: "Run the package signing service (holds the key so build workers stay keyless)",
		Args:  cliutil.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			configFile, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			cfg, err := mikoconfig.LoadMikoConfig(cmd.Flags(), configFile)
			if err != nil {
				return err
			}
			return app.RunSigner(cmd.Context(), cfg)
		},
	}
}
