package repository

import (
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv/badgerkv"
)

func newTestDenylistRepo(t *testing.T) DenylistRepository {
	t.Helper()
	store, err := badgerkv.New(t.TempDir())
	if err != nil {
		t.Fatalf("badgerkv.New: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return NewDenylistRepository(store)
}

func TestDenylistRevokeAndCheck(t *testing.T) {
	r := newTestDenylistRepo(t)

	if r.IsRevoked("") {
		t.Fatal("empty jti must never be revoked")
	}
	if r.IsRevoked("unknown") {
		t.Fatal("an un-revoked jti must not read as revoked")
	}

	if err := r.Revoke("jti-1", time.Hour); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if !r.IsRevoked("jti-1") {
		t.Fatal("jti-1 must read as revoked after Revoke")
	}
	if r.IsRevoked("jti-2") {
		t.Fatal("revoking jti-1 must not revoke jti-2")
	}
}

func TestDenylistRejectsEmptyAndExpired(t *testing.T) {
	r := newTestDenylistRepo(t)

	if err := r.Revoke("", time.Hour); err == nil {
		t.Fatal("Revoke with an empty jti must error")
	}
	// A non-positive ttl means the token has already expired, so revoking is a
	// no-op rather than a permanent (ttl==0 => no-expiry) denylist entry.
	if err := r.Revoke("expired", 0); err != nil {
		t.Fatalf("Revoke with ttl=0 should be a no-op, got: %v", err)
	}
	if r.IsRevoked("expired") {
		t.Fatal("a ttl<=0 revoke must not store a permanent denylist entry")
	}
}
