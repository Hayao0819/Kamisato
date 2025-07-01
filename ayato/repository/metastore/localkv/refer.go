package localkv

import (
	"github.com/dgraph-io/badger/v3"
	"github.com/Hayao0819/Kamisato/internal/utils"
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
		return utils.WrapErr(err, "failed to get item")
	})
	if err != nil {
		return "", utils.WrapErr(err, "badger.PackageFile view")
	}

	return string(dstBuf), nil
}
