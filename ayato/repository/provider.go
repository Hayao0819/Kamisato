package repository

import (
	"github.com/BrenekH/blinky"
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

// PkgNameStoreProvider is a provider for storing package names and file names.
type PkgNameStoreProvider blinky.PackageNameToFileProvider

// PkgBinaryStoreProvider is an interface for binary file stores.
type PkgBinaryStoreProvider interface {
	// StoreFile saves a file.
	StoreFile(repo string, arch string, stream stream.IFileSeekStream) error

	// StoreFileWithSignedURL saves a file with a signed URL.
	StoreFileWithSignedURL(repo string, arch string, name string) (string, error)

	// DeleteFile deletes a file from the database.
	DeleteFile(repo string, arch string, file string) error

	// FetchFile retrieves a file from the database.
	FetchFile(repo string, arch string, file string) (stream.IFileStream, error)

	// RepoAdd はリポジトリにパッケージを追加します。
	RepoAdd(name string, arch string, pkg, sig stream.IFileSeekStream, useSignedDB bool, gnupgDir *string) error

	// RepoRemove はリポジトリからパッケージを削除します。
	RepoRemove(name string, arch string, pkg string, useSignedDB bool, gnupgDir *string) error

	// Init はデータベースを初期化します。
	Init(name string, arch string, useSignedDB bool, gnupgDir *string) error

	// RepoNames は全リポジトリ名を返します。
	RepoNames() ([]string, error)

	// Files はリポジトリ内のファイル一覧を返します。
	Files(repo string, arch string) ([]string, error)

	// Arches はリポジトリのアーキテクチャ一覧を返します。
	Arches(repo string) ([]string, error)
}
