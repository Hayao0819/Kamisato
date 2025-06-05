package conf

type S3Config struct {
	Region          string `koanf:"region"` // 例: "ap-northeast-1", "auto"
	AccessKeyID     string `koanf:"accesskeyid"`
	SecretAccessKey string `koanf:"secretkey"`
	SessionToken    string `koanf:"sessiontoken"` // 任意
	Bucket          string `koanf:"bucket"`       // バケット名
	Endpoint        string `koanf:"endpoint"`     // 例: "https://<account_id>.r2.cloudflarestorage.com"
	UsePathStyle    bool   `koanf:"usepathstyle"` // R2では true を推奨
}
