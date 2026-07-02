package conf

import (
	"fmt"
	"os"
	"slices"
)

// Validate rejects unknown db_type/storage_type values so a typo fails loudly
// instead of silently falling through to a default backend.
func (c StoreConfig) Validate() error {
	if !slices.Contains([]string{"", "badgerdb", "cfkv", "sql"}, c.DBType) {
		return fmt.Errorf("store.db_type: unknown value %q (want badgerdb, cfkv or sql)", c.DBType)
	}
	if !slices.Contains([]string{"", "localfs", "s3"}, c.StorageType) {
		return fmt.Errorf("store.storage_type: unknown value %q (want localfs or s3)", c.StorageType)
	}
	return nil
}

// UnderCloudRun reports whether the process runs on Cloud Run, which injects
// K_SERVICE and K_REVISION into every container.
func UnderCloudRun() bool {
	return os.Getenv("K_SERVICE") != "" || os.Getenv("K_REVISION") != ""
}

// checkStateless rejects a local-disk backend when running under Cloud Run,
// where the container filesystem is ephemeral and anything written to it is lost
// on the next revision — a silent violation of ayato's stateless invariant.
// underCloudRun is a parameter so the check stays pure and testable.
func (c StoreConfig) checkStateless(underCloudRun bool) error {
	if !underCloudRun {
		return nil
	}
	if c.DBType == "" || c.DBType == "badgerdb" {
		return fmt.Errorf("store.db_type %q is a local disk backend and is ephemeral under Cloud Run: set it to cfkv or sql", c.DBType)
	}
	if c.StorageType == "" || c.StorageType == "localfs" {
		return fmt.Errorf("store.storage_type %q is a local disk backend and is ephemeral under Cloud Run: set it to s3", c.StorageType)
	}
	return nil
}

// StoreConfig describes how application data and binaries are stored.
type StoreConfig struct {
	// DBType selects the shared generic key-value store backend
	// ("badgerdb" | "cfkv" | "sql"). That one store holds package metadata and
	// other app data, namespaced per consumer — it is not package-specific.
	// Under Cloud Run the local "badgerdb" backend is ephemeral, so cfkv or sql
	// is required there (enforced by checkStateless).
	DBType       string     `koanf:"db_type"`
	CloudflareKV CFKVConfig `koanf:"cfkv"`
	SQL          SqlConfig  `koanf:"sql"`      // config for the SQL key-value backend
	BadgerDB     string     `koanf:"badgerdb"` // base dir for the embedded BadgerDB

	// StorageType selects how binaries (package files and DBs) are stored; this
	// is independent of DBType. Under Cloud Run "localfs" is ephemeral, so s3 is
	// required there (enforced by checkStateless).
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
