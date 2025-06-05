package conf

type StoreConfig struct {
	// メタデータの保存方法
	DBType       string     `koanf:"dbtype"` // "external", "cfkv" or "badgerdb"
	CloudflareKV CFKVConfig `koanf:"cfkv"`
	SQL          SqlConfig  `koanf:"sql"` // メタデータをSQLデータベースに保存する場合の設定
	BadgerDB     string     `koanf:"badgerdb"`

	// バイナリの保存方法
	StorageType  string   `koanf:"storagetype"`  // "localfs" or "s3"
	AWSS3        S3Config `koanf:"awss3"`        // バイナリをS3に保存する場合の設定
	LocalRepoDir string   `koanf:"localrepodir"` // バイナリをローカルに保存する場合のディレクトリ
}
