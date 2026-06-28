package conf

// StoreConfig describes how application data and binaries are stored.
type StoreConfig struct {
	// DBType selects the shared generic key-value store backend
	// ("badgerdb" | "cfkv" | "sql"). That one store holds package metadata and
	// other app data, namespaced per consumer — it is not package-specific.
	DBType       string     `koanf:"dbtype"`
	CloudflareKV CFKVConfig `koanf:"cfkv"`
	SQL          SqlConfig  `koanf:"sql"`      // config for the SQL key-value backend
	BadgerDB     string     `koanf:"badgerdb"` // base dir for the embedded BadgerDB

	// StorageType selects how binaries (package files and DBs) are stored; this
	// is independent of DBType.
	StorageType  string   `koanf:"storagetype"` // "localfs" or "s3"
	AWSS3        S3Config `koanf:"awss3"`
	LocalRepoDir string   `koanf:"localrepodir"`
}

// CFKVConfig is the Cloudflare Workers KV configuration.
type CFKVConfig struct {
	Namespace string `koanf:"namespace"`
	AccountId string `koanf:"accountid"`
	Token     string `koanf:"token"`
}

// S3Config is the configuration for S3/R2-compatible storage.
type S3Config struct {
	Region          string `koanf:"region"` // e.g. "ap-northeast-1", "auto"
	AccessKeyID     string `koanf:"accesskeyid"`
	SecretAccessKey string `koanf:"secretkey"`
	SessionToken    string `koanf:"sessiontoken"` // optional
	Bucket          string `koanf:"bucket"`       // bucket name
	Endpoint        string `koanf:"endpoint"`     // e.g. "https://<account_id>.r2.cloudflarestorage.com"
	UsePathStyle    bool   `koanf:"usepathstyle"` // recommended to be true for R2
}
