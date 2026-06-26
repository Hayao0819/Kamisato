package ayato

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/saraproto"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

func TestSourceSync(t *testing.T) {
	cat := saraproto.Catalog{
		Packages: []aurweb.Pkg{{Name: "x", PackageBase: "x", Version: "1.0-1"}},
		Sources:  map[string]string{"x": "https://git.example.com/x.git"},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != catalogPath {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(cat)
	}))
	defer ts.Close()

	s := New("test", ts.URL)
	ctx := context.Background()
	if err := s.Sync(ctx); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	info, _ := s.Info(ctx, []string{"x"})
	if len(info) != 1 || info[0].Version != "1.0-1" {
		t.Fatalf("Info = %+v", info)
	}
	if u, ok, _ := s.SourceURL(ctx, "x"); !ok || u != "https://git.example.com/x.git" {
		t.Errorf("SourceURL = %q ok=%v", u, ok)
	}
	if sug, _ := s.Suggest(ctx, "x", false); len(sug) != 1 || sug[0] != "x" {
		t.Errorf("Suggest = %v", sug)
	}
}
