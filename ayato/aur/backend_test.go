package aur

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
	"github.com/gin-gonic/gin"
)

type memKV struct {
	data map[string]map[string][]byte
}

func newMemKV() *memKV { return &memKV{data: map[string]map[string][]byte{}} }

func (m *memKV) Get(ns, key string) ([]byte, error) {
	if v, ok := m.data[ns][key]; ok {
		return v, nil
	}
	return nil, kv.ErrNotFound
}

func (m *memKV) Set(ns, key string, value []byte, _ time.Duration) error {
	if m.data[ns] == nil {
		m.data[ns] = map[string][]byte{}
	}
	m.data[ns][key] = value
	return nil
}

func (m *memKV) Delete(ns, key string) error {
	delete(m.data[ns], key)
	return nil
}

func (m *memKV) List(ns string) ([]kv.Entry, error) {
	var out []kv.Entry
	for k, v := range m.data[ns] {
		out = append(out, kv.Entry{Key: k, Value: v})
	}
	return out, nil
}

func (m *memKV) Close() error { return nil }

// countingKV counts List fan-outs so a test can prove the catalog cache spares
// repeated builds.
type countingKV struct {
	*memKV
	listCalls int
}

func (m *countingKV) List(ns string) ([]kv.Entry, error) {
	m.listCalls++
	return m.memKV.List(ns)
}

func TestCatalogHandlerCachesEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := newMemKV()
	if err := m.Set(nsPkg, "foo", []byte(`{"Name":"foo"}`), 0); err != nil {
		t.Fatal(err)
	}
	if err := m.Set(nsBase, "foo", []byte(`{"url":"https://example.test/foo.git","names":["foo"]}`), 0); err != nil {
		t.Fatal(err)
	}
	ck := &countingKV{memKV: m}
	h := NewHandler(New(ck, "maint"), nil, time.Minute)

	call := func() {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/catalog", nil)
		h.CatalogHandler(c)
		if w.Code != http.StatusOK {
			t.Fatalf("catalog status = %d, want 200", w.Code)
		}
	}

	call()
	afterFirst := ck.listCalls
	if afterFirst == 0 {
		t.Fatal("first catalog build should hit the KV store")
	}
	call()
	if ck.listCalls != afterFirst {
		t.Fatalf("cached catalog still fanned out to KV: %d list calls after second hit, want %d", ck.listCalls, afterFirst)
	}
}

func initGitRepo(t *testing.T, srcinfo string) string {
	t.Helper()
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("PKGBUILD", "# pkgbuild\n")
	write(".SRCINFO", srcinfo)

	for _, args := range [][]string{
		{"init", "--quiet"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
		{"add", "-A"},
		{"commit", "--quiet", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	return dir
}

const srcinfoFixture = `
pkgbase = privtool
	pkgver = 2.0.0
	pkgrel = 1
	arch = x86_64
	depends = glibc

pkgname = privtool
	pkgdesc = a private tool
`

func TestRegisterQueryRemove(t *testing.T) {
	repo := initGitRepo(t, srcinfoFixture)
	b := New(newMemKV(), "ops@example")
	ctx := context.Background()

	// ingest is the parse/store half of Register; the clone half is exercised by
	// gitcmd's tests (strict validation rejects local-path repos like this one).
	pkgbase, names, err := b.ingest(ctx, repo, repo, "")
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if pkgbase != "privtool" || len(names) != 1 || names[0] != "privtool" {
		t.Fatalf("ingest returned pkgbase=%q names=%v", pkgbase, names)
	}

	info, _ := b.Info(ctx, []string{"privtool"})
	if len(info) != 1 || info[0].Version != "2.0.0-1" {
		t.Fatalf("Info = %+v", info)
	}
	if info[0].Maintainer != "ops@example" {
		t.Errorf("default maintainer not applied: %q", info[0].Maintainer)
	}

	found, _ := b.Search(ctx, aurweb.ByNameDesc, "private")
	if len(found) != 1 {
		t.Errorf("Search by desc = %d, want 1", len(found))
	}

	target, ok, _ := b.SourceURL(ctx, "privtool")
	if !ok || target != repo {
		t.Errorf("SourceURL = %q ok=%v, want %q", target, ok, repo)
	}

	sug, _ := b.Suggest(ctx, "priv", false)
	if len(sug) != 1 || sug[0] != "privtool" {
		t.Errorf("Suggest = %v", sug)
	}

	if err := b.Remove(ctx, "privtool"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if info, _ := b.Info(ctx, []string{"privtool"}); len(info) != 0 {
		t.Errorf("package still present after Remove: %+v", info)
	}
	if _, ok, _ := b.SourceURL(ctx, "privtool"); ok {
		t.Errorf("pkgbase still registered after Remove")
	}
}

func TestRegisterRejectsUnsafeURL(t *testing.T) {
	b := New(newMemKV(), "ops@example")
	for _, url := range []string{"file:///etc/passwd", "ext::sh -c id", "https://169.254.169.254/x"} {
		if _, _, err := b.Register(context.Background(), url, "", ""); err == nil {
			t.Errorf("Register(%q) = nil, want rejected", url)
		}
	}
}
