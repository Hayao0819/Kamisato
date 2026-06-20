// File: https://github.com/BrenekH/blinky/blob/dc156eb662a6f52ab98c41ea792af17ed2e66b8a/keyvaluestore/kv.go
package localkv

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/ayato/repository/namestore/localkv/logger"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/dgraph-io/badger/v3"
)

// Badger は BadgerDB ベースの NameStore 実装です。
// implements: github.com/BrenekH/blinky.PackageNameToFileProvider
type Badger struct {
	db *badger.DB
}

func NewBadger(dir string) (*Badger, error) {
	opt := badger.DefaultOptions(dir)
	opt.Logger = logger.Default()

	db, err := badger.Open(opt)
	if err != nil {
		return nil, err
	}
	return &Badger{
		db: db,
	}, nil
}

func (b *Badger) StorePackageFile(packageName, filePath string) error {
	key := []byte(packageName)
	val := []byte(filePath)

	err := b.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, val)
	})
	if err != nil {
		return fmt.Errorf("badger.StorePackageFile update: %w", err)
	}

	return nil
}

func (b *Badger) PackageFile(packageName string) (string, error) {
	key := []byte(packageName)

	var dstBuf []byte
	err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}

		dstBuf, err = item.ValueCopy(nil)
		return utils.WrapErr(err, "failed to get item")
	})
	if err != nil {
		return "", utils.WrapErr(err, "badger.PackageFile view")
	}

	return string(dstBuf), nil
}

func (b *Badger) DeletePackageFileEntry(packageName string) error {
	key := []byte(packageName)

	err := b.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
	if err != nil {
		return fmt.Errorf("badger.DeletePackageFileEntry update: %w", err)
	}

	return nil
}
