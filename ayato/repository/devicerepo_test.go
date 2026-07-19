package repository

import (
	"sync"
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

	if ok, err := r.ApproveDevice("US-ER", 42, "alice"); err != nil || !ok {
		t.Fatalf("ApproveDevice ok=%v err=%v", ok, err)
	}
	status, id, login, ok, err := r.PollDevice("dc")
	if err != nil || !ok || status != auth.DeviceApproved || id != 42 || login != "alice" {
		t.Fatalf("after approve: status=%q id=%d login=%q ok=%v err=%v", status, id, login, ok, err)
	}

	if consumed, err := r.ConsumeDevice("dc"); err != nil || !consumed {
		t.Fatalf("ConsumeDevice consumed=%v err=%v", consumed, err)
	}
	if _, _, _, ok, _ := r.PollDevice("dc"); ok {
		t.Fatal("a consumed device_code must not poll")
	}
	if _, ok, _ := r.LookupByUserCode("US-ER"); ok {
		t.Fatal("consuming must drop the user-code index too")
	}
}

func TestDeviceConsumeIsAtomic(t *testing.T) {
	r := newTestDeviceRepo(t)
	if err := r.CreateDevice("dc-race", "RA-CE", time.Hour); err != nil {
		t.Fatal(err)
	}
	const contenders = 16
	start := make(chan struct{})
	results := make(chan bool, contenders)
	var wait sync.WaitGroup
	for range contenders {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			consumed, err := r.ConsumeDevice("dc-race")
			results <- err == nil && consumed
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	winners := 0
	for won := range results {
		if won {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("successful consumers = %d, want exactly 1", winners)
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
