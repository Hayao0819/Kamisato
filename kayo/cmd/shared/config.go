package shared

import (
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/spf13/cobra"
)

func LoadConfig(cmd *cobra.Command) (*conf.KayoConfig, error) {
	configFile, _ := cmd.Flags().GetString("config")
	return conf.LoadKayoConfig(cmd.Flags(), configFile)
}
