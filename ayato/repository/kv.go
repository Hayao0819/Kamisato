// Fike: https://github.com/BrenekH/blinky/blob/dc156eb662a6f52ab98c41ea792af17ed2e66b8a/keyvaluestore/kv.go
package repository

import (
	"fmt"

	"github.com/dgraph-io/badger/v3"
)

func New(dbDirPath string) (BadgerAdapter, error) {
	db, err := badger.Open(badger.DefaultOptions(dbDirPath))
	if err != nil {
		return BadgerAdapter{}, err
	}

	return BadgerAdapter{db: db}, nil
}

type BadgerAdapter struct { // implements: github.com/BrenekH/blinky.PackageNameToFileProvider
	db *badger.DB
}

func (b *BadgerAdapter) PackageFile(packageName string) (string, error) {
	// Convert to bytes outside the txn to reduce time spent in txn.
	key := []byte(packageName)

	var dstBuf []byte
	err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}

		dstBuf, err = item.ValueCopy(nil)
		return err
	})
	if err != nil {
		return "", fmt.Errorf("badger.PackageFile view: %w", err)
	}

	return string(dstBuf), nil
}

func (b *BadgerAdapter) StorePackageFile(packageName, filePath string) error {
	// Convert to bytes outside the txn to reduce time spent in txn.
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

func (b *BadgerAdapter) DeletePackageFileEntry(packageName string) error {
	// Convert to bytes outside the txn to reduce time spent in txn.
	key := []byte(packageName)

	err := b.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
	if err != nil {
		return fmt.Errorf("badger.DeletePackageFileEntry update: %w", err)
	}

	return nil
}
