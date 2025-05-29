package localkv

import (
	"fmt"

	"github.com/dgraph-io/badger/v3"
)

func (b *Badger) PackageFile(packageName string) (string, error) {
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
