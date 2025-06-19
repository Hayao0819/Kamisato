package localkv

import (
	"fmt"

	"github.com/cockroachdb/errors"
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
		return errors.Wrap(err, "failed to get item")
	})
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("badger.PackageFile view"))
	}

	return string(dstBuf), nil
}
