package securekv

import (
	"slices"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
)

const (
	capabilityAdder = 1 << iota
	capabilityBulk
	capabilityAuditor
)

func wrapCapabilities(core *store) kv.Store {
	capabilities := 0
	if _, ok := core.inner.(kv.Adder); ok {
		capabilities |= capabilityAdder
	}
	if _, ok := core.inner.(kv.BulkStore); ok {
		capabilities |= capabilityBulk
	}
	if _, ok := core.inner.(kv.KeyAuditor); ok {
		capabilities |= capabilityAuditor
	}
	adder := adderCapability{store: core}
	bulk := bulkCapability{store: core}
	auditor := auditorCapability{store: core}
	switch capabilities {
	case capabilityAdder:
		return &adder
	case capabilityBulk:
		return &bulk
	case capabilityAuditor:
		return &auditor
	case capabilityAdder | capabilityBulk:
		return &adderBulkStore{
			store:           core,
			adderCapability: adder,
			bulkCapability:  bulk,
		}
	case capabilityAdder | capabilityAuditor:
		return &adderAuditorStore{
			store:             core,
			adderCapability:   adder,
			auditorCapability: auditor,
		}
	case capabilityBulk | capabilityAuditor:
		return &bulkAuditorStore{
			store:             core,
			bulkCapability:    bulk,
			auditorCapability: auditor,
		}
	case capabilityAdder | capabilityBulk | capabilityAuditor:
		return &fullStore{
			store:             core,
			adderCapability:   adder,
			bulkCapability:    bulk,
			auditorCapability: auditor,
		}
	default:
		return core
	}
}

// The three capability decorators implement each optional interface once.
// Combination types embed them while directly embedding store to keep the base
// kv.Store methods unambiguous.
type adderCapability struct{ *store }

var _ kv.Adder = (*adderCapability)(nil)

func (s *adderCapability) Add(
	namespace, key string,
	value []byte,
	ttl time.Duration,
) (bool, error) {
	value, err := s.seal(namespace, value)
	if err != nil {
		return false, err
	}
	return s.inner.(kv.Adder).Add(namespace, key, value, ttl)
}

type bulkCapability struct{ *store }

var _ kv.BulkStore = (*bulkCapability)(nil)

func (s *bulkCapability) BulkSet(
	namespace string,
	entries []kv.Entry,
	ttl time.Duration,
) error {
	if !s.encrypts(namespace) {
		return s.inner.(kv.BulkStore).BulkSet(namespace, entries, ttl)
	}
	sealedEntries := slices.Clone(entries)
	for index := range sealedEntries {
		sealed, err := s.seal(namespace, entries[index].Value)
		if err != nil {
			return err
		}
		sealedEntries[index].Value = sealed
	}
	return s.inner.(kv.BulkStore).BulkSet(namespace, sealedEntries, ttl)
}

func (s *bulkCapability) BulkDelete(namespace string, keys []string) error {
	return s.inner.(kv.BulkStore).BulkDelete(namespace, keys)
}

type auditorCapability struct{ *store }

var _ kv.KeyAuditor = (*auditorCapability)(nil)

func (s *auditorCapability) ForeignKeys() ([]string, error) {
	return s.inner.(kv.KeyAuditor).ForeignKeys()
}

func (s *auditorCapability) DeleteRawKeys(keys []string) error {
	return s.inner.(kv.KeyAuditor).DeleteRawKeys(keys)
}

type adderBulkStore struct {
	*store
	adderCapability
	bulkCapability
}

var (
	_ kv.Adder     = (*adderBulkStore)(nil)
	_ kv.BulkStore = (*adderBulkStore)(nil)
)

type adderAuditorStore struct {
	*store
	adderCapability
	auditorCapability
}

var (
	_ kv.Adder      = (*adderAuditorStore)(nil)
	_ kv.KeyAuditor = (*adderAuditorStore)(nil)
)

type bulkAuditorStore struct {
	*store
	bulkCapability
	auditorCapability
}

var (
	_ kv.BulkStore  = (*bulkAuditorStore)(nil)
	_ kv.KeyAuditor = (*bulkAuditorStore)(nil)
)

type fullStore struct {
	*store
	adderCapability
	bulkCapability
	auditorCapability
}

var (
	_ kv.Adder      = (*fullStore)(nil)
	_ kv.BulkStore  = (*fullStore)(nil)
	_ kv.KeyAuditor = (*fullStore)(nil)
)
