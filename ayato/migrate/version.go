// Package migrate runs ordered, forward-only, idempotent data migrations over
// ayato's stores as a one-shot job, separate from the serving path.
package migrate

import (
	"strconv"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/schema"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

// SupportedMin/Max are the layout versions this binary can read. 0 is fresh or
// pre-un-pool; 1 is the direct layout after un-pool. Above Max means the data is
// newer than the binary understands.
const (
	SupportedMin = 0
	SupportedMax = 1
)

// ReadLayout returns the stored layout version; unset is the 0 baseline.
func ReadLayout(s kv.Store) (int, error) {
	v, err := s.Get(schema.MigrationMetadata, schema.LayoutVersionKey)
	if errors.Is(err, kv.ErrNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(string(v))
	if err != nil {
		return 0, errors.WrapErr(err, "parse layout version")
	}
	return n, nil
}

func WriteLayout(s kv.Store, version int) error {
	return s.Set(schema.MigrationMetadata, schema.LayoutVersionKey, []byte(strconv.Itoa(version)), 0)
}

// Guard reports the stored version and whether a binary reading layouts [min, max]
// can serve it; the caller decides whether an out-of-range version warns or fails.
func Guard(s kv.Store, min, max int) (version int, inRange bool, err error) {
	version, err = ReadLayout(s)
	if err != nil {
		return 0, false, err
	}
	return version, version >= min && version <= max, nil
}
