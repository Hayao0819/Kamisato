package ayatosrc

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPinStoreWriteFailureRollsBackMemory(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := pin{KeyID: "old", LastIssued: time.Unix(100, 0)}
	store := &PinStore{
		path: filepath.Join(blocker, "known_ayato.json"),
		data: map[string]pin{"source": old},
	}

	if err := store.Put("source", pin{KeyID: "new"}); err == nil {
		t.Fatal("Put = nil, want persistence error")
	}
	if got, _ := store.Get("source"); got != old {
		t.Fatalf("failed Put left in-memory pin %#v, want %#v", got, old)
	}

	if err := store.SetLastIssued("new-source", time.Unix(200, 0)); err == nil {
		t.Fatal("SetLastIssued = nil, want persistence error")
	}
	if _, ok := store.Get("new-source"); ok {
		t.Fatal("failed SetLastIssued left a new in-memory pin")
	}
}
