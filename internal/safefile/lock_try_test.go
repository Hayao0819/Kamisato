package safefile

import "testing"

func TestTryLockFailsImmediatelyOnContention(t *testing.T) {
	path := t.TempDir() + "/output.lock"
	first, err := TryLock(path, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := first.Unlock(); err != nil {
			t.Errorf("unlock: %v", err)
		}
	}()
	if _, err := TryLock(path, 0o600); err == nil {
		t.Fatal("contended TryLock succeeded")
	}
}
