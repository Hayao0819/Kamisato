package ayatosrc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/pkg/atomicfile"
)

// pin records the TOFU-trusted public key and the anti-rollback watermark for one
// ayato source. Explicitly-pinned sources keep PubKey empty (the key lives in
// config); TOFU sources persist the first-seen key here.
type pin struct {
	PubKey     string    `json:"pubkey,omitempty"`
	KeyID      string    `json:"key_id,omitempty"`
	FirstSeen  time.Time `json:"first_seen,omitzero"`
	LastSeen   time.Time `json:"last_seen,omitzero"`
	LastIssued time.Time `json:"last_issued,omitzero"` // highest accepted IssuedAt (rollback floor)
}

// PinStore persists ayato pins to known_ayato.json. Safe for concurrent use.
type PinStore struct {
	path string
	mu   sync.Mutex
	data map[string]pin // key = source name
}

// OpenPinStore loads the pin store, creating an empty one if absent.
func OpenPinStore(path string) (*PinStore, error) {
	s := &PinStore{path: path, data: map[string]pin{}}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, errors.WrapErr(err, "failed to read ayato pin store")
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		return nil, errors.WrapErr(err, "corrupt ayato pin store")
	}
	if s.data == nil {
		s.data = map[string]pin{}
	}
	return s, nil
}

func (s *PinStore) Get(name string) (pin, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.data[name]
	return p, ok
}

func (s *PinStore) Put(name string, p pin) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Never let a re-pin regress the anti-rollback watermark: carry forward the
	// higher of the existing and incoming LastIssued.
	old, existed := s.data[name]
	if old.LastIssued.After(p.LastIssued) {
		p.LastIssued = old.LastIssued
	}
	s.data[name] = p
	if err := s.saveLocked(); err != nil {
		if existed {
			s.data[name] = old
		} else {
			delete(s.data, name)
		}
		return err
	}
	return nil
}

// SetLastIssued advances the anti-rollback watermark (monotonic).
func (s *PinStore) SetLastIssued(name string, t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, existed := s.data[name]
	p := old
	if !t.After(p.LastIssued) {
		return nil
	}
	p.LastIssued = t
	p.LastSeen = time.Now()
	s.data[name] = p
	if err := s.saveLocked(); err != nil {
		if existed {
			s.data[name] = old
		} else {
			delete(s.data, name)
		}
		return err
	}
	return nil
}

type PinInfo struct {
	Name       string
	KeyID      string
	PubKey     string
	FirstSeen  time.Time
	LastSeen   time.Time
	LastIssued time.Time
}

func (s *PinStore) Entries() []PinInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]PinInfo, 0, len(s.data))
	for name, p := range s.data {
		out = append(out, PinInfo{
			Name:       name,
			KeyID:      p.KeyID,
			PubKey:     p.PubKey,
			FirstSeen:  p.FirstSeen,
			LastSeen:   p.LastSeen,
			LastIssued: p.LastIssued,
		})
	}
	slices.SortFunc(out, func(a, b PinInfo) int { return strings.Compare(a.Name, b.Name) })
	return out
}

func (s *PinStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o750); err != nil {
		return errors.WrapErr(err, "failed to create pin store dir")
	}
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return errors.WrapErr(err, "failed to encode pin store")
	}
	// Callers treat SetLastIssued success as "reached disk" before swapping the
	// index, so both the bytes and the rename must be durable.
	return errors.WrapErr(atomicfile.WriteFile(s.path, raw, 0o600), "failed to save ayato pin store")
}
