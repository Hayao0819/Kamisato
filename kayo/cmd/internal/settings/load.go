package settings

import (
	"github.com/spf13/pflag"

	kayoconfig "github.com/Hayao0819/Kamisato/kayo/config"
)

func Load(flags *pflag.FlagSet) (*kayoconfig.KayoConfig, error) {
	configFile, err := flags.GetString("config")
	if err != nil {
		return nil, err
	}
	return kayoconfig.LoadKayoConfig(flags, configFile)
}
