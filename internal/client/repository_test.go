package client

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
)

func emptyRepositoryDatabase(t *testing.T) []byte {
	t.Helper()
	var database bytes.Buffer
	compressor := gzip.NewWriter(&database)
	archive := tar.NewWriter(compressor)
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressor.Close(); err != nil {
		t.Fatal(err)
	}
	return database.Bytes()
}

func TestRepositoryDatabaseUsesCanonicalPublicEndpoint(t *testing.T) {
	t.Parallel()
	body := emptyRepositoryDatabase(t)
	httpClient := testHTTPClient(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet {
			t.Fatalf("method = %q", request.Method)
		}
		wantPath := "/proxy/root/repo/extra%2Ftesting/x86_64/extra%2Ftesting.db"
		if request.URL.EscapedPath() != wantPath {
			t.Fatalf("path = %q, want %q", request.URL.EscapedPath(), wantPath)
		}
		if request.Header.Get("Accept") != "application/octet-stream" {
			t.Fatalf("Accept = %q", request.Header.Get("Accept"))
		}
		if hasCredential(request.Header) {
			t.Fatalf("public repository request carried credentials: %#v", request.Header)
		}
		response := testResponse(http.StatusOK, string(body))
		response.ContentLength = int64(len(body))
		return response, nil
	})

	repositories, err := NewRepository(
		"https://ayato.example/proxy/root",
		WithHTTPClient(httpClient),
		WithReadAttempts(1),
	)
	if err != nil {
		t.Fatal(err)
	}
	database, err := repositories.Database(context.Background(), "extra/testing", "x86_64")
	if err != nil {
		t.Fatal(err)
	}
	if database.Name != "extra/testing" || len(database.Pkgs) != 0 {
		t.Fatalf("database = %#v", database)
	}
}

func TestRepositoryDatabaseTreatsNotFoundAsEmpty(t *testing.T) {
	t.Parallel()
	httpClient := testHTTPClient(func(*http.Request) (*http.Response, error) {
		return testResponse(http.StatusNotFound, `{"error":"repository not found"}`), nil
	})
	repositories, err := NewRepository("https://ayato.example", WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}

	database, err := repositories.Database(context.Background(), "new-repo", "aarch64")
	if err != nil {
		t.Fatal(err)
	}
	if database.Name != "new-repo" || len(database.Pkgs) != 0 {
		t.Fatalf("database = %#v, want named empty repository", database)
	}
}

func TestRepositoryDatabasePreservesProtocolFailures(t *testing.T) {
	t.Parallel()
	httpClient := testHTTPClient(func(*http.Request) (*http.Response, error) {
		return testResponse(http.StatusForbidden, `{"error":"denied","reason":"policy"}`), nil
	})
	repositories, err := NewRepository(
		"https://ayato.example",
		WithHTTPClient(httpClient),
		WithReadAttempts(1),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = repositories.Database(context.Background(), "extra", "x86_64")
	var responseErr *ResponseError
	if !errors.As(err, &responseErr) || responseErr.StatusCode != http.StatusForbidden {
		t.Fatalf("error = %v, want forbidden ResponseError", err)
	}
}

func TestReadBytesRejectsDeclaredOversizeBeforeReading(t *testing.T) {
	t.Parallel()
	body := &readCountingCloser{}
	httpClient := testHTTPClient(func(*http.Request) (*http.Response, error) {
		response := testResponse(http.StatusOK, "")
		response.Body = body
		response.ContentLength = 5
		return response, nil
	})
	transport, err := newTransport(
		"https://ayato.example",
		noCredential{},
		WithHTTPClient(httpClient),
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := transport.readBytes(
		context.Background(),
		transport.endpoint("large"),
		4,
		"fetch test data",
		"",
	); err == nil {
		t.Fatal("oversized response unexpectedly succeeded")
	}
	if body.reads != 0 || !body.closed {
		t.Fatalf("body reads/closed = %d/%t, want 0/true", body.reads, body.closed)
	}
}

type readCountingCloser struct {
	reads  int
	closed bool
}

func (r *readCountingCloser) Read([]byte) (int, error) {
	r.reads++
	return 0, io.EOF
}

func (r *readCountingCloser) Close() error {
	r.closed = true
	return nil
}
