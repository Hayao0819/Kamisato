package source

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScaffoldRejectsEscapingNamesBeforeWriting(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../existing", "nested/repo", "/absolute"} {
		t.Run(name, func(t *testing.T) {
			target := t.TempDir()
			if _, err := Scaffold(target, name, "", filepath.Join(target, "out")); err == nil {
				t.Fatalf("accepted unsafe repository name %q", name)
			}
			entries, err := os.ReadDir(target)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("invalid input wrote scaffold files: %v", entries)
			}
		})
	}
}
