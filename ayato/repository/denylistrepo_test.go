package repository

import (
	"sync"
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

func TestDenylistConsumeIsAtomic(t *testing.T) {
	repository := newTestDenylistRepo(t)
	const contenders = 8
	start := make(chan struct{})
	results := make(chan bool, contenders)
	var wait sync.WaitGroup
	for range contenders {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			consumed, err := repository.Consume("one-time-jti", time.Hour)
			results <- err == nil && consumed
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	winners := 0
	for consumed := range results {
		if consumed {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("successful consumers = %d, want exactly 1", winners)
	}
}

func TestReplayGuardRejectsNonPositiveTTL(t *testing.T) {
	store, err := badgerkv.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("close replay store: %v", err)
		}
	}()
	guard := NewReplayGuard(store)
	for _, ttl := range []time.Duration{0, -time.Nanosecond} {
		if first, err := guard.Consume("expired", ttl); err != nil || first {
			t.Fatalf("Consume ttl=%v = (%v, %v), want false, nil", ttl, first, err)
		}
	}
}

func TestDenylistRevokeAndCheck(t *testing.T) {
	r := newTestDenylistRepo(t)

	if revoked, err := r.IsRevoked(""); err != nil || revoked {
		t.Fatal("empty jti must never be revoked")
	}
	if revoked, err := r.IsRevoked("unknown"); err != nil || revoked {
		t.Fatal("an un-revoked jti must not read as revoked")
	}

	if err := r.Revoke("jti-1", time.Hour); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if revoked, err := r.IsRevoked("jti-1"); err != nil || !revoked {
		t.Fatal("jti-1 must read as revoked after Revoke")
	}
	if revoked, err := r.IsRevoked("jti-2"); err != nil || revoked {
		t.Fatal("revoking jti-1 must not revoke jti-2")
	}
}

func TestDenylistSessionFamilyIsIndependentFromTokenJTI(t *testing.T) {
	r := newTestDenylistRepo(t)
	if err := r.RevokeSession("session-1", time.Hour); err != nil {
		t.Fatal(err)
	}
	if revoked, err := r.IsSessionRevoked("session-1"); err != nil || !revoked {
		t.Fatalf("session family revoke = (%v, %v), want true, nil", revoked, err)
	}
	if revoked, err := r.IsRevoked("session-1"); err != nil || revoked {
		t.Fatal("session namespace must not revoke an unrelated token JTI")
	}
}

func TestDenylistRejectsEmptyAndExpired(t *testing.T) {
	r := newTestDenylistRepo(t)

	if err := r.Revoke("", time.Hour); err == nil {
		t.Fatal("Revoke with an empty jti must error")
	}
	if err := r.RevokeSession("", time.Hour); err == nil {
		t.Fatal("RevokeSession with an empty id must error")
	}
	// A non-positive ttl means the token has already expired, so revoking is a
	// no-op rather than a permanent (ttl==0 => no-expiry) denylist entry.
	if err := r.Revoke("expired", 0); err != nil {
		t.Fatalf("Revoke with ttl=0 should be a no-op, got: %v", err)
	}
	if revoked, err := r.IsRevoked("expired"); err != nil || revoked {
		t.Fatal("a ttl<=0 revoke must not store a permanent denylist entry")
	}
}
