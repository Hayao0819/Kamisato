package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"

	depend "github.com/Hayao0819/Kamisato/internal/pacman"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

// fakeAURInfo answers Info from a fixed name->maintainer table so the trust gate
// tests need no network. A name absent from the table returns no record, which
// the gate reads as orphaned.
type fakeAURInfo struct {
	maintainers map[string]string
	err         error
}

func (f fakeAURInfo) Info(_ context.Context, names []string) ([]aurweb.Pkg, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []aurweb.Pkg
	for _, n := range names {
		m, ok := f.maintainers[n]
		if !ok {
			continue
		}
		out = append(out, aurweb.Pkg{Name: n, PackageBase: n, Maintainer: m})
	}
	return out, nil
}

func newTrustService(t *testing.T, settings Settings) *Service {
	t.Helper()
	return New(settings)
}

func TestCheckDepTrust(t *testing.T) {
	dep := depend.Pkg{Name: "foo", PackageBase: "foo"}
	lookup := fakeAURInfo{maintainers: map[string]string{"foo": "alice"}}

	tests := []struct {
		name      string
		trust     Settings
		lookup    maintainerLookup
		wantBlock bool
	}{
		{
			name:   "trusted maintainer builds",
			trust:  Settings{TrustedAURMaintainers: []string{"alice"}},
			lookup: lookup,
		},
		{
			name:   "trusted maintainer is case-insensitive",
			trust:  Settings{TrustedAURMaintainers: []string{"ALICE"}},
			lookup: lookup,
		},
		{
			name:   "trusted pkgbase builds",
			trust:  Settings{TrustedAURPackages: []string{"foo"}},
			lookup: lookup,
		},
		{
			name:      "untrusted with allow_untrusted=false is blocked",
			trust:     Settings{TrustedAURMaintainers: []string{"bob"}},
			lookup:    lookup,
			wantBlock: true,
		},
		{
			name:   "untrusted with allow_untrusted=true is allowed",
			trust:  Settings{AllowUntrustedAURPackages: true},
			lookup: lookup,
		},
		{
			name:      "orphaned dep is blocked",
			trust:     Settings{TrustedAURMaintainers: []string{"alice"}},
			lookup:    fakeAURInfo{maintainers: map[string]string{"foo": ""}},
			wantBlock: true,
		},
		{
			name:   "orphaned dep passes via pkgbase allowlist",
			trust:  Settings{TrustedAURPackages: []string{"foo"}},
			lookup: fakeAURInfo{maintainers: map[string]string{"foo": ""}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTrustService(t, tt.trust)
			err := s.checkDepTrust(context.Background(), tt.lookup, dep)
			if tt.wantBlock {
				if err == nil {
					t.Fatal("expected the dep to be blocked, got nil")
				}
				if !strings.Contains(err.Error(), "is not trusted") || !strings.Contains(err.Error(), dep.PackageBase) {
					t.Errorf("block error should name the pkgbase and be a needs-review message, got: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected the dep to build, got: %v", err)
			}
		})
	}
}

// A missing AUR record (package gone from the AUR) is treated as orphaned and
// blocked under the secure default, not silently allowed.
func TestCheckDepTrustMissingRecordBlocked(t *testing.T) {
	s := newTrustService(t, Settings{TrustedAURMaintainers: []string{"alice"}})
	dep := depend.Pkg{Name: "ghost", PackageBase: "ghost"}
	if err := s.checkDepTrust(context.Background(), fakeAURInfo{maintainers: map[string]string{}}, dep); err == nil {
		t.Fatal("a dep with no AUR record must be blocked, got nil")
	}
}

// A lookup error must fail the build closed, not pass.
func TestCheckDepTrustLookupError(t *testing.T) {
	s := newTrustService(t, Settings{AllowUntrustedAURPackages: true})
	dep := depend.Pkg{Name: "foo", PackageBase: "foo"}
	if err := s.checkDepTrust(context.Background(), fakeAURInfo{err: errors.New("boom")}, dep); err == nil {
		t.Fatal("a lookup error must stop the build, got nil")
	}
}
