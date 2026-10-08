package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

type stagedUploadServer struct {
	mu             sync.Mutex
	presignStatus  int
	presignCalls   int
	presignBodies  []string
	puts           map[string]string
	putHadAuth     bool
	commits        []string
	multipartCalls int
}

func newStagedUploadServer(t *testing.T, presignStatus int) (*stagedUploadServer, *httptest.Server) {
	t.Helper()
	state := &stagedUploadServer{presignStatus: presignStatus, puts: map[string]string{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state.mu.Lock()
		defer state.mu.Unlock()
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/unstable/repos/core/packages/presign":
			state.presignCalls++
			body, _ := io.ReadAll(r.Body)
			state.presignBodies = append(state.presignBodies, string(body))
			if state.presignStatus != http.StatusOK {
				w.WriteHeader(state.presignStatus)
				_, _ = w.Write([]byte(`{"error":"presigned upload is disabled"}`))
				return
			}
			var req struct {
				Files []stagedFileRequest `json:"files"`
			}
			if err := json.Unmarshal(body, &req); err != nil {
				t.Errorf("decode presign request: %v", err)
			}
			urls := make(map[string]string, len(req.Files))
			for _, file := range req.Files {
				urls[file.Name] = "http://" + r.Host + "/staged/" + file.Name
			}
			_ = json.NewEncoder(w).Encode(stagedUploadGrant{
				ID:         "0123456789abcdef",
				TTLSeconds: 3600,
				URLs:       urls,
			})
		case r.Method == http.MethodPut:
			if r.Header.Get("Authorization") != "" || r.Header.Get("X-API-Key") != "" {
				state.putHadAuth = true
			}
			body, _ := io.ReadAll(r.Body)
			state.puts[r.URL.Path] = string(body)
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Path == "/api/unstable/repos/core/packages/commit":
			body, _ := io.ReadAll(r.Body)
			state.commits = append(state.commits, string(body))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("1 package(s) committed!"))
		case r.Method == http.MethodPost && r.URL.Path == "/api/unstable/repos/core/packages":
			state.multipartCalls++
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("1 package(s) uploaded!"))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return state, server
}

func writeStagedPackage(t *testing.T, withSignature bool) string {
	t.Helper()
	packagePath := filepath.Join(t.TempDir(), "demo-1-1-x86_64.pkg.tar.zst")
	if err := os.WriteFile(packagePath, []byte("package"), 0o600); err != nil {
		t.Fatal(err)
	}
	if withSignature {
		if err := os.WriteFile(packagePath+".sig", []byte("signature"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return packagePath
}

func TestStagedUploadPresignsPutsAndCommits(t *testing.T) {
	t.Parallel()
	state, server := newStagedUploadServer(t, http.StatusOK)
	packagePath := writeStagedPackage(t, true)

	publisher, err := NewPublisher(server.URL, "publisher-key")
	if err != nil {
		t.Fatal(err)
	}
	if err := publisher.UploadPackageFiles(context.Background(), "core", packagePath); err != nil {
		t.Fatal(err)
	}

	if state.presignCalls != 1 {
		t.Fatalf("presign calls = %d, want 1", state.presignCalls)
	}
	var presigned struct {
		Files []stagedFileRequest `json:"files"`
	}
	if err := json.Unmarshal([]byte(state.presignBodies[0]), &presigned); err != nil {
		t.Fatal(err)
	}
	wantFiles := []stagedFileRequest{
		{Name: "demo-1-1-x86_64.pkg.tar.zst", Size: int64(len("package"))},
		{Name: "demo-1-1-x86_64.pkg.tar.zst.sig", Size: int64(len("signature"))},
	}
	if !reflect.DeepEqual(presigned.Files, wantFiles) {
		t.Fatalf("presign files = %#v, want %#v", presigned.Files, wantFiles)
	}
	wantPuts := map[string]string{
		"/staged/demo-1-1-x86_64.pkg.tar.zst":     "package",
		"/staged/demo-1-1-x86_64.pkg.tar.zst.sig": "signature",
	}
	if !reflect.DeepEqual(state.puts, wantPuts) {
		t.Fatalf("puts = %#v, want %#v", state.puts, wantPuts)
	}
	if state.putHadAuth {
		t.Fatal("presigned PUT carried a credential header")
	}
	wantCommit := `{"id":"0123456789abcdef","files":[{"package":"demo-1-1-x86_64.pkg.tar.zst","signature":"demo-1-1-x86_64.pkg.tar.zst.sig"}]}`
	if len(state.commits) != 1 || state.commits[0] != wantCommit {
		t.Fatalf("commits = %#v, want [%s]", state.commits, wantCommit)
	}
	if state.multipartCalls != 0 {
		t.Fatalf("multipart calls = %d, want 0", state.multipartCalls)
	}
}

func TestStagedUploadFallsBackToMultipart(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		status int
	}{
		{name: "tombstoned server", status: http.StatusNotImplemented},
		{name: "older server", status: http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			state, server := newStagedUploadServer(t, tc.status)
			directory := t.TempDir()
			var packages []string
			for _, name := range []string{"one-1-1-x86_64.pkg.tar.zst", "two-1-1-x86_64.pkg.tar.zst"} {
				path := filepath.Join(directory, name)
				if err := os.WriteFile(path, []byte("package"), 0o600); err != nil {
					t.Fatal(err)
				}
				packages = append(packages, path)
			}

			publisher, err := NewPublisher(server.URL, "publisher-key")
			if err != nil {
				t.Fatal(err)
			}
			if err := publisher.UploadPackageFiles(context.Background(), "core", packages...); err != nil {
				t.Fatal(err)
			}

			// The unsupported answer is remembered: one probe, then multipart only.
			if state.presignCalls != 1 {
				t.Fatalf("presign calls = %d, want 1", state.presignCalls)
			}
			if state.multipartCalls != 2 {
				t.Fatalf("multipart calls = %d, want 2", state.multipartCalls)
			}
			if len(state.commits) != 0 || len(state.puts) != 0 {
				t.Fatalf("staged requests after fallback: commits = %v, puts = %v", state.commits, state.puts)
			}
		})
	}
}

func TestStagedUploadSurfacesCommitFailure(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/unstable/repos/core/packages/presign":
			_ = json.NewEncoder(w).Encode(stagedUploadGrant{
				ID:         "0123456789abcdef",
				TTLSeconds: 3600,
				URLs: map[string]string{
					"demo-1-1-x86_64.pkg.tar.zst": "http://" + r.Host + "/staged/demo-1-1-x86_64.pkg.tar.zst",
				},
			})
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid upload"}`))
		}
	}))
	t.Cleanup(server.Close)
	packagePath := writeStagedPackage(t, false)

	publisher, err := NewPublisher(server.URL, "publisher-key")
	if err != nil {
		t.Fatal(err)
	}
	err = publisher.UploadPackageFiles(context.Background(), "core", packagePath)
	var respErr *ResponseError
	if !errors.As(err, &respErr) || respErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("commit failure = %v, want a 400 ResponseError", err)
	}
}
