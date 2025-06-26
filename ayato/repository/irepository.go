package repository

import (
	"github.com/Hayao0819/Kamisato/ayato/stream"
	remote "github.com/Hayao0819/Kamisato/pkg/alpm/remoterepo"
)

//go:generate mockgen -source=irepository.go -destination=../test/mocks/repository.go -package=mocks
type IRepository interface {
	// DBファイル取得
	FetchDB(repoName, archName string) (stream.IFileStream, error)

	// パッケージ名一覧取得
	PkgNames(repoName, archName string) ([]string, error)

	// リモートリポジトリ取得
	RemoteRepo(name, arch string) (*remote.RemoteRepo, error)

	// パッケージファイル一覧取得
	PkgFiles(repoName, archName, pkgName string) ([]string, error)

	// ファイル操作・リポジトリ操作
	StoreFile(repo string, arch string, stream stream.IFileSeekStream) error
	StoreFileWithSignedURL(repo string, arch string, name string) (string, error)
	DeleteFile(repo string, arch string, file string) error
	RepoNames() ([]string, error)
	Files(name string, arch string) ([]string, error)
	Arches(repo string) ([]string, error)
	FetchFile(repo string, arch string, file string) (stream.IFileStream, error)
	RepoAdd(name string, arch string, pkg stream.IFileSeekStream, sig stream.IFileSeekStream, useSignedDB bool, gnupgDir *string) error
	RepoRemove(name string, arch string, pkg string, useSignedDB bool, gnupgDir *string) error

	// メタストア操作
	GetPkgFileName(name string) (fp string, err error)
	StorePkgFileName(packageName, filePath string) error
	DeletePkgFileName(packageName string) error

	// 初期化・検証
	Init(name string, useSignedDB bool, gnupgDir *string) error
	VerifyPkgRepo(name string) error
}
