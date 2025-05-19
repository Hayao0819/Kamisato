package raiou_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/raiou"
)

func TestParseAllDescFiles(t *testing.T) {
	const localDBPath = "/var/lib/pacman/local"

	entries, err := os.ReadDir(localDBPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", localDBPath, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		descPath := filepath.Join(localDBPath, entry.Name(), "desc")
		data, err := os.ReadFile(descPath)
		if err != nil {
			t.Errorf("failed to read %s: %v", descPath, err)
			continue
		}

		desc, err := raiou.ParseDescString(string(data))
		if err != nil {
			t.Errorf("failed to parse %s: %v", descPath, err)
			continue
		}

		// 未知のキーがある場合はテスト失敗
		if len(desc.ExtraFields) > 0 {
			t.Errorf("unknown keys found in %s: %v", descPath, keysOf(desc.ExtraFields))
		}
	}
}

// keysOf extracts keys from a map[string]string into a []string
func keysOf[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
