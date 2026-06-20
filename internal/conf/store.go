package conf

// StoreConfig はメタデータとバイナリの保存方式を表します。
type StoreConfig struct {
	// メタデータの保存方法
	DBType       string     `koanf:"dbtype"` // "sql", "cfkv" or "badgerdb"
	CloudflareKV CFKVConfig `koanf:"cfkv"`
	SQL          SqlConfig  `koanf:"sql"` // メタデータをSQLデータベースに保存する場合の設定
	BadgerDB     string     `koanf:"badgerdb"`

	// バイナリの保存方法
	StorageType  string   `koanf:"storagetype"`  // "localfs" or "s3"
	AWSS3        S3Config `koanf:"awss3"`        // バイナリをS3に保存する場合の設定
	LocalRepoDir string   `koanf:"localrepodir"` // バイナリをローカルに保存する場合のディレクトリ
}

// CFKVConfig は Cloudflare Workers KV の設定です。
type CFKVConfig struct {
	Namespace string `koanf:"namespace"`
	AccountId string `koanf:"accountid"`
	Token     string `koanf:"token"`
}

// S3Config は S3/R2 互換ストレージの設定です。
type S3Config struct {
	Region          string `koanf:"region"` // 例: "ap-northeast-1", "auto"
	AccessKeyID     string `koanf:"accesskeyid"`
	SecretAccessKey string `koanf:"secretkey"`
	SessionToken    string `koanf:"sessiontoken"` // 任意
	Bucket          string `koanf:"bucket"`       // バケット名
	Endpoint        string `koanf:"endpoint"`     // 例: "https://<account_id>.r2.cloudflarestorage.com"
	UsePathStyle    bool   `koanf:"usepathstyle"` // R2では true を推奨
}
