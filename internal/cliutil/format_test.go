package cliutil

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func resolveFormatFor(t *testing.T, args []string, def string) string {
	t.Helper()
	cmd := &cobra.Command{Use: "x", RunE: func(*cobra.Command, []string) error { return nil }}
	AddFormatFlags(cmd)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got, err := ResolveFormat(cmd, def)
	if err != nil {
		t.Fatalf("ResolveFormat: %v", err)
	}
	return got
}

func TestResolveFormat(t *testing.T) {
	if got := resolveFormatFor(t, nil, "table {{.A}}"); got != "table {{.A}}" {
		t.Errorf("default not applied: %q", got)
	}
	if got := resolveFormatFor(t, []string{"--json"}, "table {{.A}}"); got != "json" {
		t.Errorf("--json = %q, want json", got)
	}
	// --json wins over an explicit --format.
	if got := resolveFormatFor(t, []string{"--json", "--format", "{{.A}}"}, ""); got != "json" {
		t.Errorf("--json over --format = %q, want json", got)
	}
	if got := resolveFormatFor(t, []string{"--format", "{{.A}}"}, "table"); got != "{{.A}}" {
		t.Errorf("--format = %q, want {{.A}}", got)
	}
}

type kv struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// RenderList must work for any row type, not just PkgRow.
func TestRenderListGeneric(t *testing.T) {
	rows := []kv{{Name: "foo", Value: "1"}, {Name: "bar", Value: "2"}}
	header := kv{Name: "NAME", Value: "VALUE"}

	var table bytes.Buffer
	if err := RenderList(&table, "table {{.Name}}\t{{.Value}}", header, rows); err != nil {
		t.Fatalf("table render: %v", err)
	}
	lines := strings.Split(strings.TrimRight(table.String(), "\n"), "\n")
	if len(lines) != 3 || !strings.Contains(lines[0], "NAME") || !strings.Contains(lines[1], "foo") {
		t.Errorf("table output unexpected: %q", table.String())
	}

	var js bytes.Buffer
	if err := RenderList(&js, "json", header, rows); err != nil {
		t.Fatalf("json render: %v", err)
	}
	jsLines := strings.Split(strings.TrimRight(js.String(), "\n"), "\n")
	if len(jsLines) != 2 || !strings.Contains(jsLines[0], `"name":"foo"`) {
		t.Errorf("json output unexpected: %q", js.String())
	}
}
