package ayatocmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/internal/conf"
)

func TestAyatoListHasFormatFlags(t *testing.T) {
	cmd := ayatoListCmd()
	if cmd.Flags().Lookup("format") == nil {
		t.Error("ayato list should have --format flag")
	}
	if cmd.Flags().Lookup("json") == nil {
		t.Error("ayato list should have --json flag")
	}
}

func TestAyatoListTableOutput(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "kayo_config.toml")
	if err := os.WriteFile(cfgPath, []byte("addr = \":10713\"\ntrust_store = \""+filepath.Join(dir, "trust.json")+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	root := Cmd()
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.PersistentFlags().String("config", cfgPath, "")
	root.SetArgs([]string{"list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("ayato list: %v (out: %s)", err, buf.String())
	}
	// With no sources configured, the header row should still appear.
	if !strings.Contains(buf.String(), "KIND") {
		t.Errorf("ayato list output missing table header: %q", buf.String())
	}
}

func TestSourceMode(t *testing.T) {
	tests := []struct {
		src  conf.AyatoSource
		want string
	}{
		{conf.AyatoSource{Insecure: true}, "insecure"},
		{conf.AyatoSource{PubKey: "abc", Trust: "delegate"}, "delegate"},
		{conf.AyatoSource{PubKey: "abc"}, "pinned"},
		{conf.AyatoSource{TrustOnFirstUse: true}, "first-use"},
		{conf.AyatoSource{}, "review"},
	}
	for _, tt := range tests {
		if got := sourceMode(tt.src); got != tt.want {
			t.Errorf("sourceMode(%+v) = %q, want %q", tt.src, got, tt.want)
		}
	}
}

func TestKeyOrDash(t *testing.T) {
	if got := keyOrDash(""); got != "-" {
		t.Errorf("keyOrDash(\"\") = %q, want \"-\"", got)
	}
	if got := keyOrDash("abc123"); got != "abc123" {
		t.Errorf("keyOrDash(\"abc123\") = %q, want \"abc123\"", got)
	}
}

func TestWatermark(t *testing.T) {
	if got := watermark(time.Time{}); got != "-" {
		t.Errorf("watermark(zero) = %q, want \"-\"", got)
	}
	ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	want := "2024-01-02T03:04:05Z"
	if got := watermark(ts); got != want {
		t.Errorf("watermark(%v) = %q, want %q", ts, got, want)
	}
}
