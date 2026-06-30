package conf

// StoreConfig describes how application data and binaries are stored.
type StoreConfig struct {
	// DBType selects the shared generic key-value store backend
	// ("badgerdb" | "cfkv" | "sql"). That one store holds package metadata and
	// other app data, namespaced per consumer — it is not package-specific.
	DBType       string     `koanf:"db_type"`
	CloudflareKV CFKVConfig `koanf:"cfkv"`
	SQL          SqlConfig  `koanf:"sql"`      // config for the SQL key-value backend
	BadgerDB     string     `koanf:"badgerdb"` // base dir for the embedded BadgerDB

	// StorageType selects how binaries (package files and DBs) are stored; this
	// is independent of DBType.
	StorageType  string   `koanf:"storage_type"` // "localfs" or "s3"
	AWSS3        S3Config `koanf:"awss3"`
	LocalRepoDir string   `koanf:"local_repo_dir"`
}

// CFKVConfig is the Cloudflare Workers KV configuration.
type CFKVConfig struct {
	Namespace string `koanf:"namespace"`
	AccountId string `koanf:"account_id"`
	Token     string `koanf:"token"`
}

// S3Config is the configuration for S3/R2-compatible storage.
type S3Config struct {
	Region          string `koanf:"region"` // e.g. "ap-northeast-1", "auto"
	AccessKeyID     string `koanf:"access_key_id"`
	SecretAccessKey string `koanf:"secret_access_key"`
	SessionToken    string `koanf:"session_token"`  // optional
	Bucket          string `koanf:"bucket"`         // bucket name
	Endpoint        string `koanf:"endpoint"`       // e.g. "https://<account_id>.r2.cloudflarestorage.com"
	UsePathStyle    bool   `koanf:"use_path_style"` // recommended to be true for R2
}
