package ayatocmd

import (
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ayato",
		Short: "Inspect federated ayato sources and their pinned signing keys",
	}
	cmd.AddCommand(ayatoListCmd(), ayatoPinCmd())
	return cmd
}
