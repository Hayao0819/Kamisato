package kv

import (
	"fmt"

	"github.com/dgraph-io/badger/v3"
)

func (b *Badger) DeletePackageFileEntry(packageName string) error {
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
