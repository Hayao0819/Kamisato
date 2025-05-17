package repository

import "github.com/BrenekH/blinky"

type PkgNameStoreProvider blinky.PackageNameToFileProvider
type PkgBinaryStoreProvider interface {
	DbAddR(name string, packageName string, useSignedDB bool, gnupgDir *string) error
	DbRemove(name string, packageName string, useSignedDB bool, gnupgDir *string) error
	DbInit(usesignedDB bool, gnupgDir *string) error
	RepoNames() ([]string, error)
	FileList(name string, arch string) ([]string, error)
	VerifyPkgRepo(name string) error
}
