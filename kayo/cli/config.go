package cli

import (
	"github.com/spf13/cobra"

	kayoconfig "github.com/Hayao0819/Kamisato/kayo/config"
)

func LoadConfig(cmd *cobra.Command) (*kayoconfig.KayoConfig, error) {
	configFile, _ := cmd.Flags().GetString("config")
	return kayoconfig.LoadKayoConfig(cmd.Flags(), configFile)
}
