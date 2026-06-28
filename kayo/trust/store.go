// Package trust is kayo's local trust store: the whitelist of approved packages
// (pinned to a reviewed commit, recorded under the maintainer ACCOUNT that owned
// them at review time) and the set of maintainer accounts the user vouches for.
//
// The anchor is the maintainer account, never a git commit email: aurweb
// authenticates the pushing account (SSH key) but does not validate commit
// author/committer email, so email is attacker-settable and untrustworthy. The
// trustworthy identity is the RPC Maintainer (account username), namespaced per
// source so an account on one source can't impersonate the same name on another.
package trust

import (
	"cmp"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/Hayao0819/Kamisato/internal/utils"
)

// TrustedMaintainer is a maintainer account the user explicitly vouches for,
// scoped to a source. Trust is a heuristic to be revisited, not proof.
type TrustedMaintainer struct {
	Source  string    `json:"source"` // "aur" | ayato name; overlays are trusted by config
	Account string    `json:"account"`
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
		data: data{Maintainers: map[string]TrustedMaintainer{}, Approvals: map[string]Approval{}},
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, utils.WrapErr(err, "failed to read trust store")
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		return nil, utils.WrapErr(err, "corrupt trust store")
	}
	if s.data.Maintainers == nil {
		s.data.Maintainers = map[string]TrustedMaintainer{}
	}
	if s.data.Approvals == nil {
		s.data.Approvals = map[string]Approval{}
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

// UntrustMaintainer revokes a maintainer.
func (s *Store) UntrustMaintainer(source, account string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Maintainers, mkey(source, account))
}

// IsMaintainerTrusted reports whether account is vouched for on source.
func (s *Store) IsMaintainerTrusted(source, account string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.data.Maintainers[mkey(source, account)]
	return ok
}

// Approve records (or updates) a package approval.
func (s *Store) Approve(a Approval) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a.ApprovedAt.IsZero() {
		a.ApprovedAt = time.Now()
	}
	s.data.Approvals[a.Pkgbase] = a
}

// Approval returns the approval for a pkgbase, if any.
func (s *Store) Approval(pkgbase string) (Approval, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.data.Approvals[pkgbase]
	return a, ok
}

// RemoveApproval drops a package approval.
func (s *Store) RemoveApproval(pkgbase string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Approvals, pkgbase)
}

// Maintainers returns the trusted maintainers, sorted.
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

// Approvals returns the approved packages, sorted by pkgbase.
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

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return utils.WrapErr(err, "failed to create trust store dir")
	}
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return utils.WrapErr(err, "failed to encode trust store")
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return utils.WrapErr(err, "failed to write trust store")
	}
	return os.Rename(tmp, s.path)
}
