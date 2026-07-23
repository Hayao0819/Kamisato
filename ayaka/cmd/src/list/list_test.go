package listcmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/ayaka/service/report"
)

func renderToString(t *testing.T, format string, rows []report.Row) string {
	t.Helper()
	var buf bytes.Buffer
	if err := renderRows(&buf, format, rows); err != nil {
		t.Fatalf("renderRows(%q) error: %v", format, err)
	}
	return buf.String()
}

func TestRenderRowsFormats(t *testing.T) {
	rows := []report.Row{
		{Package: "foo", Installed: "1.0-1", Local: "1.1-1", Remote: "1.0-1", Build: "success"},
		{Package: "bar", Installed: "-", Local: "2.0-1", Remote: "-", Build: "queued"},
	}

	t.Run("default table has header and aligns", func(t *testing.T) {
		out := renderToString(t, report.DefaultListFormat, rows)
		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		if len(lines) != 3 {
			t.Fatalf("want 3 lines (header + 2 rows), got %d: %q", len(lines), out)
		}
		header := lines[0]
		for _, col := range []string{"PACKAGE", "INSTALLED", "LOCAL", "REMOTE", "BUILD"} {
			if !strings.Contains(header, col) {
				t.Errorf("header missing column %q: %q", col, header)
			}
		}
		if !strings.Contains(lines[1], "foo") || !strings.Contains(lines[1], "success") {
			t.Errorf("row 1 unexpected: %q", lines[1])
		}
	})

	t.Run("custom table selects columns", func(t *testing.T) {
		out := renderToString(t, `table {{.Package}}\t{{.Local}}`, rows)
		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		if !strings.Contains(lines[0], "PACKAGE") || !strings.Contains(lines[0], "LOCAL") {
			t.Errorf("header unexpected: %q", lines[0])
		}
		if strings.Contains(lines[0], "REMOTE") {
			t.Errorf("header should not contain REMOTE: %q", lines[0])
		}
	})

	t.Run("plain template has no header", func(t *testing.T) {
		out := renderToString(t, `{{.Package}} {{.Build}}`, rows)
		if strings.Contains(out, "PACKAGE") {
			t.Errorf("plain template should not print a header: %q", out)
		}
		if !strings.Contains(out, "foo success") || !strings.Contains(out, "bar queued") {
			t.Errorf("plain output unexpected: %q", out)
		}
	})

	t.Run("json emits one object per line", func(t *testing.T) {
		out := renderToString(t, "json", rows)
		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		if len(lines) != 2 {
			t.Fatalf("want 2 json lines, got %d: %q", len(lines), out)
		}
		if !strings.Contains(lines[0], `"package":"foo"`) {
			t.Errorf("json line 1 unexpected: %q", lines[0])
		}
	})

	t.Run("invalid template errors", func(t *testing.T) {
		var buf bytes.Buffer
		if err := renderRows(&buf, `{{.Nope`, rows); err == nil {
			t.Error("expected error for malformed template, got nil")
		}
	})
}
