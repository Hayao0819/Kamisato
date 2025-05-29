package conf

type CFKVConfig struct {
	Namespace string `koanf:"namespace"`
	AccountId string `koanf:"accountid"`
	Token     string `koanf:"token"`
}
