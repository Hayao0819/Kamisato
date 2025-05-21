package provider

import (
	"io"

	"github.com/BrenekH/blinky"
)

type PkgNameStoreProvider blinky.PackageNameToFileProvider

type PkgBinaryStoreProvider interface {
	// StoreFile stores a file
	StoreFile(repo string, arch string, file string, useSignedDB bool, gnupgDir *string) error

	// DeleteFile deletes a file from the database
	DeleteFile(repo string, arch string, file string, useSignedDB bool, gnupgDir *string) error

	FetchFile(repo string, arch string, file string) (BinaryStream, error)

	// Init initializes the database.
	Init(name string, arch string, useSignedDB bool, gnupgDir *string) error
	RepoNames() ([]string, error)
	Files(repo string, arch string) ([]string, error)
	Arches(repo string) ([]string, error)
}

// 多分ここにあっちゃだめ
type BinaryStream interface {
	io.ReadCloser        // ストリーミング返却
	FileName() string    // ダウンロード時のファイル名
	ContentType() string // MIMEタイプ（例: application/zip）
}
