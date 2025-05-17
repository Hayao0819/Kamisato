// File: https://github.com/BrenekH/blinky/blob/dc156eb662a6f52ab98c41ea792af17ed2e66b8a/keyvaluestore/kv.go
package kv

import (
	"github.com/dgraph-io/badger/v3"
)

// implements: github.com/BrenekH/blinky.PackageNameToFileProvider
// implements: github.com/Hayao0819/Kamisato/ayato/repository.PkgNameStoreProvider
type Badger struct {
	db *badger.DB
}

func NewBadger(dir string) (*Badger, error) {
	db, err := badger.Open(badger.DefaultOptions(dir))
	if err != nil {
		return nil, err
	}
	return &Badger{
		db: db,
	}, nil
}
