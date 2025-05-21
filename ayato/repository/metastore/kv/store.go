package kv

import (
	"fmt"

	"github.com/dgraph-io/badger/v3"
)

func (b *Badger) StorePackageFile(packageName, filePath string) error {
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
