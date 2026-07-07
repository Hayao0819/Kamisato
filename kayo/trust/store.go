// Package trust is kayo's local trust store: approved packages (each pinned to a
// reviewed commit under the maintainer ACCOUNT that owned it) and the maintainer
// accounts the user vouches for. The anchor is the maintainer account, never a git
// commit email — aurweb authenticates the pushing account but not commit author
// email, so email is attacker-settable. Accounts are namespaced per source so a name
// on one source can't impersonate the same name on another.
package trust

import (
	"cmp"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errwrap"
)

// TrustedMaintainer is a maintainer account the user explicitly vouches for,
// scoped to a source. Trust is a heuristic to be revisited, not proof.
type TrustedMaintainer struct {
	Source  string    `json:"source"` // "aur" | ayato name; overlays are trusted by config
	Account string    `json:"account"`
	AddedAt time.Time `json:"added_at"`
	Note    string    `json:"note,omitempty"`
}

// WhitelistEntry is a pkgbase the user unconditionally auto-approves: it pins no
// commit and records no maintainer, a blanket escape hatch that skips the
// new-package inspection AND the maintainer-change check, so a maintainer swap on a
// whitelisted pkgbase is NOT caught. Blunter than `trust add`; use it only for
// packages accepted without review.
type WhitelistEntry struct {
	Pkgbase string    `json:"pkgbase"`
	AddedAt time.Time `json:"added_at"`
	Note    string    `json:"note,omitempty"`
}

// Approval records that a pkgbase was reviewed at a specific commit under a
// specific maintainer account. A later commit or a changed maintainer is a
// signal to re-review.
type Approval struct {
	Pkgbase    string    `json:"pkgbase"`
	Source     string    `json:"source"`
	Maintainer string    `json:"maintainer"` // account at review time ("" = orphan)
	Commit     string    `json:"commit"`     // full reviewed commit hash
	AuditMax   string    `json:"audit_max,omitempty"`
	ApprovedAt time.Time `json:"approved_at"`
}

type data struct {
	Maintainers map[string]TrustedMaintainer `json:"maintainers"` // key = source\x00account
	Approvals   map[string]Approval          `json:"approvals"`   // key = pkgbase
	Whitelist   map[string]WhitelistEntry    `json:"whitelist"`   // key = pkgbase
}

// Store persists trust state to a JSON file. It is safe for concurrent use.
type Store struct {
	path string
	mu   sync.Mutex
	data data
}

func mkey(source, account string) string { return source + "\x00" + account }

// Open loads the store at path, creating an empty one if absent.
func Open(path string) (*Store, error) {
	s := &Store{
		path: path,
		data: data{Maintainers: map[string]TrustedMaintainer{}, Approvals: map[string]Approval{}, Whitelist: map[string]WhitelistEntry{}},
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, errwrap.WrapErr(err, "failed to read trust store")
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		return nil, errwrap.WrapErr(err, "corrupt trust store")
	}
	if s.data.Maintainers == nil {
		s.data.Maintainers = map[string]TrustedMaintainer{}
	}
	if s.data.Approvals == nil {
		s.data.Approvals = map[string]Approval{}
	}
	if s.data.Whitelist == nil {
		s.data.Whitelist = map[string]WhitelistEntry{}
	}
	return s, nil
}

// TrustMaintainer vouches for account on source (no-op if already trusted).
func (s *Store) TrustMaintainer(source, account, note string) {
	if account == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := mkey(source, account)
	if _, ok := s.data.Maintainers[k]; ok {
		return
	}
	s.data.Maintainers[k] = TrustedMaintainer{Source: source, Account: account, AddedAt: time.Now(), Note: note}
}

func (s *Store) UntrustMaintainer(source, account string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Maintainers, mkey(source, account))
}

func (s *Store) IsMaintainerTrusted(source, account string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.data.Maintainers[mkey(source, account)]
	return ok
}

func (s *Store) Approve(a Approval) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a.ApprovedAt.IsZero() {
		a.ApprovedAt = time.Now()
	}
	s.data.Approvals[a.Pkgbase] = a
}

func (s *Store) Approval(pkgbase string) (Approval, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.data.Approvals[pkgbase]
	return a, ok
}

func (s *Store) RemoveApproval(pkgbase string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Approvals, pkgbase)
}

// AddWhitelist unconditionally auto-approves pkgbase (no-op if already listed).
func (s *Store) AddWhitelist(pkgbase, note string) {
	if pkgbase == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data.Whitelist[pkgbase]; ok {
		return
	}
	s.data.Whitelist[pkgbase] = WhitelistEntry{Pkgbase: pkgbase, AddedAt: time.Now(), Note: note}
}

func (s *Store) RemoveWhitelist(pkgbase string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Whitelist, pkgbase)
}

func (s *Store) IsWhitelisted(pkgbase string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.data.Whitelist[pkgbase]
	return ok
}

func (s *Store) WhitelistEntries() []WhitelistEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]WhitelistEntry, 0, len(s.data.Whitelist))
	for _, w := range s.data.Whitelist {
		out = append(out, w)
	}
	slices.SortFunc(out, func(a, b WhitelistEntry) int { return cmp.Compare(a.Pkgbase, b.Pkgbase) })
	return out
}

func (s *Store) Maintainers() []TrustedMaintainer {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]TrustedMaintainer, 0, len(s.data.Maintainers))
	for _, m := range s.data.Maintainers {
		out = append(out, m)
	}
	slices.SortFunc(out, func(a, b TrustedMaintainer) int {
		return cmp.Or(cmp.Compare(a.Source, b.Source), cmp.Compare(a.Account, b.Account))
	})
	return out
}

func (s *Store) Approvals() []Approval {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Approval, 0, len(s.data.Approvals))
	for _, a := range s.data.Approvals {
		out = append(out, a)
	}
	slices.SortFunc(out, func(a, b Approval) int { return cmp.Compare(a.Pkgbase, b.Pkgbase) })
	return out
}

// Save writes the store to disk atomically.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o750); err != nil {
		return errwrap.WrapErr(err, "failed to create trust store dir")
	}
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return errwrap.WrapErr(err, "failed to encode trust store")
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return errwrap.WrapErr(err, "failed to write trust store")
	}
	return os.Rename(tmp, s.path)
}
