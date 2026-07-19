package service_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/conf"
)

func TestUpstreamLayeringAndSync(t *testing.T) {
	upstream := &fakeUpstream{}
	db, files := buildRepoDB(t, "extra", []pkgSpec{
		{name: "foo", ver: "1.0-1"},
		{name: "bar", ver: "1.0-1"},
	})
	upstream.set(db, files, "v1")
	server := httptest.NewServer(upstream.handler())
	defer server.Close()

	svc, _, _ := newTieredService(t, []conf.BinRepoConfig{{
		Name:     "myrepo",
		Upstream: conf.UpstreamRepoConfig{DBURL: server.URL + "/extra.db"},
	}})
	ctx := context.Background()
	uploadVersioned(t, svc, "myrepo", "bar", "2.0-1")
	uploadVersioned(t, svc, "myrepo", "baz", "1.0-1")

	result, err := svc.SyncUpstream(ctx, "myrepo")
	if err != nil {
		t.Fatalf("SyncUpstream: %v", err)
	}
	if len(result.Arches) != 1 || !result.Arches[0].Changed {
		t.Fatalf("first sync = %+v, want one changed arch", result.Arches)
	}
	if version, ok := versionOf(t, svc, "myrepo", "x86_64", "foo"); !ok || version != "1.0-1" {
		t.Fatalf("merged foo = %q,%v, want upstream 1.0-1", version, ok)
	}
	if version, ok := versionOf(t, svc, "myrepo", "x86_64", "bar"); !ok || version != "2.0-1" {
		t.Fatalf("merged bar = %q,%v, want local 2.0-1", version, ok)
	}
	if _, ok := versionOf(t, svc, "myrepo", "x86_64", "baz"); !ok {
		t.Fatal("merged view lost local-only baz")
	}

	before := upstream.dbReadCount()
	result, err = svc.SyncUpstream(ctx, "myrepo")
	if err != nil {
		t.Fatalf("second SyncUpstream: %v", err)
	}
	if result.Arches[0].Changed {
		t.Error("unchanged upstream reported a change")
	}
	if after := upstream.dbReadCount(); after != before {
		t.Errorf("full db reads changed from %d to %d", before, after)
	}

	db, files = buildRepoDB(t, "extra", []pkgSpec{
		{name: "foo", ver: "1.0-1"},
		{name: "bar", ver: "1.0-1"},
		{name: "qux", ver: "1.0-1"},
	})
	upstream.set(db, files, "v2")
	result, err = svc.SyncUpstream(ctx, "myrepo")
	if err != nil {
		t.Fatalf("third SyncUpstream: %v", err)
	}
	if !result.Arches[0].Changed || result.Arches[0].Added != 1 {
		t.Fatalf("changed sync = %+v, want one addition", result.Arches[0])
	}
	if _, ok := versionOf(t, svc, "myrepo", "x86_64", "qux"); !ok {
		t.Fatal("merged view did not add upstream qux")
	}
	if version, ok := versionOf(t, svc, "myrepo", "x86_64", "bar"); !ok || version != "2.0-1" {
		t.Fatalf("local bar = %q,%v after sync, want 2.0-1", version, ok)
	}
	if _, ok := versionOf(t, svc, "myrepo", "x86_64", "baz"); !ok {
		t.Fatal("local-only baz did not survive sync")
	}
}

func TestSyncUpstreamRejectsNonUpstream(t *testing.T) {
	svc, _, _ := newTieredService(t, []conf.BinRepoConfig{{Name: "plain"}})
	if _, err := svc.SyncUpstream(context.Background(), "plain"); err == nil {
		t.Fatal("SyncUpstream on a non-upstream repo = nil, want error")
	}
}

func TestSyncUpstreamKeepsAddressedPhysicalTier(t *testing.T) {
	upstream := &fakeUpstream{}
	db, files := buildRepoDB(t, "extra", []pkgSpec{{name: "foo", ver: "1.0-1"}})
	upstream.set(db, files, "v1")
	server := httptest.NewServer(upstream.handler())
	defer server.Close()

	svc, _, _ := newTieredService(t, []conf.BinRepoConfig{{
		Name:     "myrepo",
		Tiered:   true,
		Arches:   []string{"x86_64"},
		Upstream: conf.UpstreamRepoConfig{DBURL: server.URL + "/extra.db"},
	}})
	result, err := svc.SyncUpstream(context.Background(), "myrepo-testing")
	if err != nil {
		t.Fatalf("SyncUpstream(testing): %v", err)
	}
	if result.Repo != "myrepo-testing" {
		t.Fatalf("result repo = %q, want addressed physical tier", result.Repo)
	}
	if _, ok := versionOf(t, svc, "myrepo-testing", "x86_64", "foo"); !ok {
		t.Fatal("testing tier did not receive the upstream snapshot")
	}
	if _, ok := versionOf(t, svc, "myrepo", "x86_64", "foo"); ok {
		t.Fatal("syncing testing unexpectedly rewrote stable")
	}
}

func TestSyncUpstreamRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Length", "536870913")
		response.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	svc, _, _ := newTieredService(t, []conf.BinRepoConfig{{
		Name: "myrepo", Upstream: conf.UpstreamRepoConfig{DBURL: server.URL + "/extra.db"},
	}})
	uploadVersioned(t, svc, "myrepo", "local", "1.0-1")
	result, err := svc.SyncUpstream(context.Background(), "myrepo")
	if err != nil {
		t.Fatalf("SyncUpstream: %v", err)
	}
	if len(result.Arches) != 1 || !strings.Contains(result.Arches[0].Error, "exceeds") {
		t.Fatalf("result = %+v, want oversized-response error", result)
	}
}
