package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/internal/pacman/builder"
)

func TestAyakaValidate(t *testing.T) {
	if err := (&AyakaConfig{}).Validate(); err != nil {
		t.Errorf("empty config (legacy/fresh) rejected: %v", err)
	}
	if err := (&AyakaConfig{Repos: []RepoEntry{{Dir: "myrepo"}}}).Validate(); err != nil {
		t.Errorf("valid repo entry rejected: %v", err)
	}
	if err := (&AyakaConfig{Repos: []RepoEntry{{DestDir: "out"}}}).Validate(); err == nil {
		t.Error("expected an error for a repo entry with no dir")
	}
}

func TestAyakaBuilderTimeoutJSON(t *testing.T) {
	config := &AyakaConfig{
		Builder: builder.HostConfig{
			Backend: builder.KindContainer,
			Timeout: 30 * time.Minute,
		},
	}
	data, err := config.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"timeout": "30m0s"`) {
		t.Fatalf("timeout was not marshalled with units:\n%s", data)
	}

	path := filepath.Join(t.TempDir(), ".ayakarc.json")
	if err := os.WriteFile(path, []byte(`{"builder":{"backend":"container","timeout":30}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAyakaConfigFrom(path, nil); err == nil {
		t.Fatal("unitless builder.timeout: want error")
	}
}
