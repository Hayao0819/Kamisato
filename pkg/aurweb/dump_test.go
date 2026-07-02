package aurweb

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

// dumpUpstream is a fakeUpstream that also serves bulk dumps, so buildMetaDump and
// buildNamesDump exercise the local-shadows-upstream merge.
type dumpUpstream struct {
	fakeUpstream
	meta  string   // JSON array text returned by DumpReader
	names []string // names returned by FetchNames
}

func (d *dumpUpstream) DumpReader(_ context.Context, _ bool) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(d.meta)), nil
}

func (d *dumpUpstream) FetchNames(_ context.Context) ([]string, error) {
	return d.names, nil
}

func gunzip(t *testing.T, b []byte) []byte {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("not gzip: %v", err)
	}
	out, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("gunzip: %v", err)
	}
	return out
}

func TestMetaDumpShadowsUpstream(t *testing.T) {
	be := &stubBackend{pkgs: map[string]Pkg{
		"shared":    {Name: "shared", PackageBase: "shared", Version: "local-1"},
		"localonly": {Name: "localonly", PackageBase: "localonly", Version: "local-1"},
	}}
	up := &dumpUpstream{
		meta:  `[{"Name":"shared","Version":"upstream-9"},{"Name":"upstreamonly","Version":"upstream-9"}]`,
		names: []string{"shared", "upstreamonly"},
	}
	s := New(be, WithUpstream(up))

	body, err := s.buildMetaDump(context.Background(), true)
	if err != nil {
		t.Fatalf("buildMetaDump: %v", err)
	}
	var arr []map[string]any
	if err := json.Unmarshal(gunzip(t, body), &arr); err != nil {
		t.Fatalf("dump not valid json: %v", err)
	}

	versions := map[string]string{}
	for _, e := range arr {
		name, _ := e["Name"].(string)
		if _, dup := versions[name]; dup {
			t.Fatalf("duplicate name %q in dump", name)
		}
		versions[name], _ = e["Version"].(string)
	}
	if len(versions) != 3 {
		t.Fatalf("dump has %d names, want 3 (shared, localonly, upstreamonly): %v", len(versions), versions)
	}
	// A local package must shadow the upstream one of the same name.
	if versions["shared"] != "local-1" {
		t.Errorf("shared came from %q, want the local record (local-1)", versions["shared"])
	}
}

func TestNamesDumpShadowsUpstream(t *testing.T) {
	be := &stubBackend{pkgs: map[string]Pkg{
		"shared":    {Name: "shared", PackageBase: "shared"},
		"localonly": {Name: "localonly", PackageBase: "localonly"},
	}}
	up := &dumpUpstream{names: []string{"shared", "upstreamonly"}}
	s := New(be, WithUpstream(up))

	body, err := s.buildNamesDump(context.Background())
	if err != nil {
		t.Fatalf("buildNamesDump: %v", err)
	}
	names := strings.Fields(string(gunzip(t, body)))

	counts := map[string]int{}
	for _, n := range names {
		counts[n]++
	}
	if counts["shared"] != 1 {
		t.Errorf("name %q appears %d times, want exactly 1 (local shadows upstream)", "shared", counts["shared"])
	}
	for _, want := range []string{"shared", "localonly", "upstreamonly"} {
		if counts[want] == 0 {
			t.Errorf("names dump missing %q", want)
		}
	}
	if len(names) != 3 {
		t.Errorf("names dump = %v, want 3 unique", names)
	}
}
