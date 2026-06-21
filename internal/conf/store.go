package conf

// StoreConfig describes how metadata and binaries are stored.
type StoreConfig struct {
	// How metadata is stored
	DBType       string     `koanf:"dbtype"` // "sql", "cfkv" or "badgerdb"
	CloudflareKV CFKVConfig `koanf:"cfkv"`
	SQL          SqlConfig  `koanf:"sql"` // config for storing metadata in an SQL database
	BadgerDB     string     `koanf:"badgerdb"`

	// How binaries are stored
	StorageType  string   `koanf:"storagetype"`  // "localfs" or "s3"
	AWSS3        S3Config `koanf:"awss3"`        // config for storing binaries in S3
	LocalRepoDir string   `koanf:"localrepodir"` // directory for storing binaries locally
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
