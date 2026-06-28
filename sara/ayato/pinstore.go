package ayato

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Hayao0819/Kamisato/internal/utils"
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
		return nil, utils.WrapErr(err, "failed to read ayato pin store")
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		return nil, utils.WrapErr(err, "corrupt ayato pin store")
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
	s.data[name] = p
	return s.saveLocked()
}

// SetLastIssued advances the anti-rollback watermark (monotonic).
func (s *PinStore) SetLastIssued(name string, t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.data[name]
	if !t.After(p.LastIssued) {
		return nil
	}
	p.LastIssued = t
	p.LastSeen = time.Now()
	s.data[name] = p
	return s.saveLocked()
}

func (s *PinStore) List() map[string]pin {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]pin, len(s.data))
	for k, v := range s.data {
		out[k] = v
	}
	return out
}

// PinInfo is a read-only view of one pinned source for display.
type PinInfo struct {
	Name       string
	KeyID      string
	PubKey     string
	FirstSeen  time.Time
	LastSeen   time.Time
	LastIssued time.Time
}

// Entries returns every pin as a display view, sorted by name.
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
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return utils.WrapErr(err, "failed to create pin store dir")
	}
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return utils.WrapErr(err, "failed to encode pin store")
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return utils.WrapErr(err, "failed to write pin store")
	}
	return os.Rename(tmp, s.path)
}
