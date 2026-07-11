// Package migrate runs ordered, forward-only, idempotent data migrations over
// ayato's blob and key-value stores as a one-shot job, separate from the serving
// path. The stored layout version gates which layouts a serving binary may read.
package migrate

import (
	"strconv"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

const (
	metaNS    = "_meta_"
	layoutKey = "layout_version"
)

// SupportedMin/Max are the layout versions this binary can read. 0 is a fresh or
// pre-migration install (no pool, or pool not yet contracted); 1 is the direct
// layout after un-pool. A stored version above Max means the data is newer than the
// binary understands.
const (
	SupportedMin = 0
	SupportedMax = 1
)

// ReadLayout returns the stored layout version; an unset version is the 0 baseline.
func ReadLayout(s kv.Store) (int, error) {
	v, err := s.Get(metaNS, layoutKey)
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

// WriteLayout records the layout version reached after a migration completes.
func WriteLayout(s kv.Store, version int) error {
	return s.Set(metaNS, layoutKey, []byte(strconv.Itoa(version)), 0)
}

// Guard returns the stored layout version and whether a binary that reads layouts
// [min, max] can serve it. The caller applies policy (c): an out-of-range version a
// migration marked incompatible fails startup; otherwise it is only a warning.
func Guard(s kv.Store, min, max int) (version int, inRange bool, err error) {
	version, err = ReadLayout(s)
	if err != nil {
		return 0, false, err
	}
	return version, version >= min && version <= max, nil
}
