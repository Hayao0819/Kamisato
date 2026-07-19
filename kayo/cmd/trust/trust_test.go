package trustcmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/kayo/trust"
	"github.com/spf13/cobra"
)

func seedStore(t *testing.T, dir string) {
	t.Helper()
	storePath := filepath.Join(dir, "trust.json")
	st, err := trust.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	st.TrustMaintainer("aur", "jguer", "")
	st.Approve(trust.Approval{Pkgbase: "yay", Source: "aur", Maintainer: "jguer", Commit: "abc123456789"})
	st.AddWhitelist("linux-zen", "")
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestTrustListTableOutput(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "kayo_config.toml")
	storePath := filepath.Join(dir, "trust.json")
	if err := os.WriteFile(cfgPath, []byte("addr = \":10713\"\ntrust_store = \""+storePath+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	seedStore(t, dir)

	var buf bytes.Buffer
	root := Cmd()
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.PersistentFlags().String("config", cfgPath, "")
	root.SetArgs([]string{"list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("trust list: %v (out: %s)", err, buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "maintainer") || !strings.Contains(out, "jguer") {
		t.Errorf("maintainer row missing: %q", out)
	}
	if !strings.Contains(out, "package") || !strings.Contains(out, "yay") {
		t.Errorf("package row missing: %q", out)
	}
	if !strings.Contains(out, "whitelist") || !strings.Contains(out, "linux-zen") {
		t.Errorf("whitelist row missing: %q", out)
	}
}

func TestTrustListJSONOutput(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "kayo_config.toml")
	storePath := filepath.Join(dir, "trust.json")
	if err := os.WriteFile(cfgPath, []byte("addr = \":10713\"\ntrust_store = \""+storePath+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	seedStore(t, dir)

	var buf bytes.Buffer
	root := Cmd()
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.PersistentFlags().String("config", cfgPath, "")
	root.SetArgs([]string{"list", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("trust list --json: %v", err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 JSON lines, got %d: %q", len(lines), buf.String())
	}
	var row trustRow
	if err := json.Unmarshal([]byte(lines[0]), &row); err != nil {
		t.Fatalf("first JSON line is not valid JSON: %v — %q", err, lines[0])
	}
	if row.Kind == "" {
		t.Errorf("JSON row missing kind field: %q", lines[0])
	}
}

func TestRemoveCommands(t *testing.T) {
	tests := []struct {
		name, use string
		cmd       func() *cobra.Command
	}{
		{"trust", "remove [<pkgname>]", trustRemoveCmd},
		{"whitelist", "remove <pkgname>", whitelistRemoveCmd},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.cmd()
			if !slices.Contains(cmd.Aliases, "rm") {
				t.Errorf("aliases = %v, want rm", cmd.Aliases)
			}
			if cmd.Use != tt.use {
				t.Errorf("Use = %q, want %q", cmd.Use, tt.use)
			}
		})
	}
}

func TestTrustListHasFormatFlags(t *testing.T) {
	cmd := trustListCmd()
	if cmd.Flags().Lookup("format") == nil {
		t.Error("trust list should have --format flag")
	}
	if cmd.Flags().Lookup("json") == nil {
		t.Error("trust list should have --json flag")
	}
}
