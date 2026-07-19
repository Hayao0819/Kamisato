package repository

import (
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

// getOptional normalizes a KV miss to ok=false while preserving backend
// failures with the repository operation that triggered them.
func getOptional(
	store kv.Store,
	namespace, key, errorContext string,
) ([]byte, bool, error) {
	value, err := store.Get(namespace, key)
	if errors.Is(err, kv.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, errors.WrapErr(err, errorContext)
	}
	return value, true, nil
}
