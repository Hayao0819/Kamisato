package repository

import (
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/badgerkv"
)

func newTestDeviceRepo(t *testing.T) DeviceRepository {
	t.Helper()
	store, err := badgerkv.New(t.TempDir())
	if err != nil {
		t.Fatalf("badgerkv.New: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return NewDeviceRepository(store)
}

func TestDeviceCreateApproveConsume(t *testing.T) {
	r := newTestDeviceRepo(t)

	if err := r.CreateDevice("dc", "US-ER", time.Hour); err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}

	status, _, _, ok, err := r.PollDevice("dc")
	if err != nil || !ok {
		t.Fatalf("PollDevice ok=%v err=%v", ok, err)
	}
	if status != auth.DevicePending {
		t.Fatalf("fresh status = %q, want pending", status)
	}

	// Approval attaches the identity and flips the status the client polls for.
	if ok, err := r.ApproveDevice("US-ER", 42, "alice"); err != nil || !ok {
		t.Fatalf("ApproveDevice ok=%v err=%v", ok, err)
	}
	status, id, login, ok, err := r.PollDevice("dc")
	if err != nil || !ok || status != auth.DeviceApproved || id != 42 || login != "alice" {
		t.Fatalf("after approve: status=%q id=%d login=%q ok=%v err=%v", status, id, login, ok, err)
	}

	// Consuming clears both the record and its user-code index.
	if err := r.ConsumeDevice("dc"); err != nil {
		t.Fatalf("ConsumeDevice: %v", err)
	}
	if _, _, _, ok, _ := r.PollDevice("dc"); ok {
		t.Fatal("a consumed device_code must not poll")
	}
	if _, ok, _ := r.LookupByUserCode("US-ER"); ok {
		t.Fatal("consuming must drop the user-code index too")
	}
}

func TestDeviceDenyAndUnknown(t *testing.T) {
	r := newTestDeviceRepo(t)

	if _, ok, err := r.LookupByUserCode("NO-PE"); err != nil || ok {
		t.Fatalf("unknown user code ok=%v err=%v, want ok=false", ok, err)
	}
	if ok, err := r.ApproveDevice("NO-PE", 1, "x"); err != nil || ok {
		t.Fatalf("approving an unknown code ok=%v err=%v, want ok=false", ok, err)
	}

	if err := r.CreateDevice("dc2", "DE-NY", time.Hour); err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}
	if ok, err := r.DenyDevice("DE-NY"); err != nil || !ok {
		t.Fatalf("DenyDevice ok=%v err=%v", ok, err)
	}
	if status, _, _, ok, _ := r.PollDevice("dc2"); !ok || status != auth.DeviceDenied {
		t.Fatalf("after deny: status=%q ok=%v, want denied", status, ok)
	}
}
