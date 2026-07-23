package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

func testHTTPClient(roundTrip roundTripFunc) *http.Client {
	return &http.Client{Transport: roundTrip}
}

func testResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestMikoUsesAPIKeyAndPreservesBasePrefix(t *testing.T) {
	t.Parallel()
	var gotPath, gotRawPath, gotAPIKey, gotBearer string
	httpClient := testHTTPClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotRawPath = req.URL.EscapedPath()
		gotAPIKey = req.Header.Get("X-API-Key")
		gotBearer = req.Header.Get("Authorization")
		return testResponse(http.StatusOK, `{"id":"a/b","status":"queued"}`), nil
	})
	client, err := NewMiko("https://builder.example/proxy/root/", "service-secret", WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.JobStatus(context.Background(), "a/b"); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/proxy/root/api/unstable/jobs/a/b" {
		t.Fatalf("decoded path = %q", gotPath)
	}
	if gotRawPath != "/proxy/root/api/unstable/jobs/a%2Fb" {
		t.Fatalf("escaped path = %q", gotRawPath)
	}
	if gotAPIKey != "service-secret" || gotBearer != "" {
		t.Fatalf("credentials = X-API-Key %q, Authorization %q", gotAPIKey, gotBearer)
	}
}

func TestReadRetriesButMutationDoesNot(t *testing.T) {
	t.Parallel()

	t.Run("read", func(t *testing.T) {
		calls := 0
		httpClient := testHTTPClient(func(*http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return testResponse(http.StatusServiceUnavailable, `{"error":"busy"}`), nil
			}
			return testResponse(http.StatusOK, `[]`), nil
		})
		client, err := NewMiko("https://builder.example", "key", WithHTTPClient(httpClient), WithReadAttempts(2))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.ListJobs(context.Background()); err != nil {
			t.Fatal(err)
		}
		if calls != 2 {
			t.Fatalf("calls = %d, want 2", calls)
		}
	})

	t.Run("mutation", func(t *testing.T) {
		calls := 0
		httpClient := testHTTPClient(func(*http.Request) (*http.Response, error) {
			calls++
			return testResponse(http.StatusServiceUnavailable, `{"error":"busy"}`), nil
		})
		client, err := NewMiko("https://builder.example", "key", WithHTTPClient(httpClient), WithReadAttempts(3))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.SubmitBuild(context.Background(), &BuildRequest{Repo: "core", Arch: "x86_64"}); err == nil {
			t.Fatal("SubmitBuild unexpectedly succeeded")
		}
		if calls != 1 {
			t.Fatalf("calls = %d, want 1", calls)
		}
	})
}

type rotatingTokenSource struct {
	mu        sync.Mutex
	token     string
	refreshes int
}

func (s *rotatingTokenSource) Token(context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.token, nil
}

func (s *rotatingTokenSource) RefreshIfCurrent(_ context.Context, stale string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != stale {
		return nil
	}
	s.token = "new-token"
	s.refreshes++
	return nil
}

func TestAuthenticatedConstructorsRejectMissingKeysAndCleartextUserEndpoint(t *testing.T) {
	t.Parallel()
	if _, err := NewMiko("https://miko.example", ""); err == nil {
		t.Fatal("NewMiko accepted an empty API key")
	}
	if _, err := NewPublisher("https://ayato.example", ""); err == nil {
		t.Fatal("NewPublisher accepted an empty API key")
	}
	if _, err := NewSigner("https://signer.example", ""); err == nil {
		t.Fatal("NewSigner accepted an empty API key")
	}
	if _, err := NewAyato("http://ayato.example", StaticBearer("user-token")); err == nil {
		t.Fatal("NewAyato accepted a clear-text non-loopback endpoint")
	}
	if _, err := NewAyato("http://127.0.0.1:8080", StaticBearer("user-token")); err != nil {
		t.Fatalf("NewAyato rejected loopback development endpoint: %v", err)
	}
}

func TestExplicitExpiryRefreshesAndReconstructsOperationOnce(t *testing.T) {
	t.Parallel()
	source := &rotatingTokenSource{token: "old-token"}
	var bodies, authorizations []string
	httpClient := testHTTPClient(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		bodies = append(bodies, string(body))
		authorizations = append(authorizations, req.Header.Get("Authorization"))
		if len(bodies) == 1 {
			resp := testResponse(http.StatusUnauthorized, `{"message":"expired"}`)
			resp.Header.Set(accessTokenExpiredHeader, "1")
			return resp, nil
		}
		return testResponse(http.StatusAccepted, `{"job_id":"job-1"}`), nil
	})
	client, err := NewAyato("https://ayato.example", source, WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	jobID, err := client.SubmitBuild(context.Background(), &BuildRequest{Repo: "core", Arch: "x86_64"})
	if err != nil {
		t.Fatal(err)
	}
	if jobID != "job-1" {
		t.Fatalf("job id = %q", jobID)
	}
	if source.refreshes != 1 {
		t.Fatalf("refreshes = %d", source.refreshes)
	}
	if !reflect.DeepEqual(authorizations, []string{"Bearer old-token", "Bearer new-token"}) {
		t.Fatalf("authorizations = %#v", authorizations)
	}
	if len(bodies) != 2 || bodies[0] != bodies[1] {
		t.Fatalf("operation body was not reconstructed: %#v", bodies)
	}
}

func TestOrdinaryUnauthorizedDoesNotRefresh(t *testing.T) {
	t.Parallel()
	source := &rotatingTokenSource{token: "old-token"}
	calls := 0
	httpClient := testHTTPClient(func(*http.Request) (*http.Response, error) {
		calls++
		return testResponse(http.StatusUnauthorized, `{"message":"denied"}`), nil
	})
	client, err := NewAyato("https://ayato.example", source, WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListJobs(context.Background()); err == nil {
		t.Fatal("ListJobs unexpectedly succeeded")
	}
	if calls != 1 || source.refreshes != 0 {
		t.Fatalf("calls = %d, refreshes = %d", calls, source.refreshes)
	}
}

func TestAuthenticatedRedirectIsNotFollowed(t *testing.T) {
	t.Parallel()
	requests := 0
	httpClient := testHTTPClient(func(req *http.Request) (*http.Response, error) {
		requests++
		if requests > 1 {
			t.Fatalf("credentialed redirect was followed to %s", req.URL)
		}
		resp := testResponse(http.StatusFound, "redirect")
		resp.Header.Set("Location", "https://attacker.example/collect")
		return resp, nil
	})
	client, err := NewAyato("https://ayato.example", StaticBearer("secret"), WithHTTPClient(httpClient), WithReadAttempts(1))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListJobs(context.Background()); err == nil {
		t.Fatal("ListJobs unexpectedly followed redirect")
	}
	if requests != 1 {
		t.Fatalf("requests = %d", requests)
	}
}

func TestPublicDownloadFollowsRedirectWithoutCredential(t *testing.T) {
	t.Parallel()
	var seen []string
	httpClient := testHTTPClient(func(req *http.Request) (*http.Response, error) {
		if hasCredential(req.Header) {
			t.Fatalf("credential leaked to public request: %#v", req.Header)
		}
		seen = append(seen, req.URL.String())
		if req.URL.Host == "ayato.example" {
			resp := testResponse(http.StatusTemporaryRedirect, "")
			resp.Header.Set("Location", "https://objects.example/package.pkg.tar.zst")
			return resp, nil
		}
		return testResponse(http.StatusOK, "package bytes"), nil
	})
	client, err := NewAyato("https://ayato.example/prefix", StaticBearer("secret"), WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	var destination bytes.Buffer
	if err := client.DownloadPackage(context.Background(), "core", "x86_64", "pkg.pkg.tar.zst", &destination); err != nil {
		t.Fatal(err)
	}
	if destination.String() != "package bytes" || len(seen) != 2 {
		t.Fatalf("destination = %q, requests = %#v", destination.String(), seen)
	}
}

func TestDownloadPackageFilePreservesDestinationOnStreamFailure(t *testing.T) {
	t.Parallel()
	streamErr := errors.New("response stream failed")
	httpClient := testHTTPClient(func(*http.Request) (*http.Response, error) {
		resp := testResponse(http.StatusOK, "")
		resp.Body = io.NopCloser(io.MultiReader(strings.NewReader("partial"), errorReader{err: streamErr}))
		return resp, nil
	})
	client, err := NewAyato("https://ayato.example", nil, WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "package.pkg.tar.zst")
	if err := os.WriteFile(destination, []byte("existing package"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := client.DownloadPackageFile(
		context.Background(),
		"core",
		"x86_64",
		"package.pkg.tar.zst",
		destination,
	); !errors.Is(err, streamErr) {
		t.Fatalf("DownloadPackageFile error = %v, want stream failure", err)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "existing package" {
		t.Fatalf("failed download replaced destination with %q", got)
	}
}

func TestMultipartUploadReconstructedAfterExplicitExpiry(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	packagePath := filepath.Join(directory, "demo-1-1-x86_64.pkg.tar.zst")
	if err := os.WriteFile(packagePath, []byte("package"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packagePath+".sig", []byte("signature"), 0o600); err != nil {
		t.Fatal(err)
	}

	source := &rotatingTokenSource{token: "old-token"}
	var uploads [][]string
	httpClient := testHTTPClient(func(req *http.Request) (*http.Response, error) {
		// Tombstone the staged-protocol probe so the flow under test stays multipart.
		if strings.HasSuffix(req.URL.Path, "/packages/presign") {
			return testResponse(http.StatusNotImplemented, `{"error":"presigned upload is disabled"}`), nil
		}
		mediaType, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
		if err != nil || mediaType != "multipart/form-data" {
			t.Fatalf("content type = %q: %v", mediaType, err)
		}
		parts, err := readMultipart(req.Body, params["boundary"])
		if err != nil {
			t.Fatal(err)
		}
		uploads = append(uploads, parts)
		if len(uploads) == 1 {
			resp := testResponse(http.StatusUnauthorized, `{"message":"expired"}`)
			resp.Header.Set(accessTokenExpiredHeader, "1")
			return resp, nil
		}
		return testResponse(http.StatusOK, `{}`), nil
	})
	client, err := NewAyato("https://ayato.example", source, WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	if err := client.UploadPackageFiles(context.Background(), "core", packagePath); err != nil {
		t.Fatal(err)
	}
	want := []string{"package:demo-1-1-x86_64.pkg.tar.zst=package", "signature:demo-1-1-x86_64.pkg.tar.zst.sig=signature"}
	if len(uploads) != 2 || !reflect.DeepEqual(uploads[0], want) || !reflect.DeepEqual(uploads[1], want) {
		t.Fatalf("uploads = %#v", uploads)
	}
}

func TestMultipartUploadRejectsOrphanSignature(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	packagePath := filepath.Join(directory, "demo.pkg.tar.zst")
	orphanSignature := filepath.Join(directory, "other.pkg.tar.zst.sig")
	if err := os.WriteFile(packagePath, []byte("package"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(orphanSignature, []byte("signature"), 0o600); err != nil {
		t.Fatal(err)
	}
	client, err := NewPublisher("https://ayato.example", "key")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.UploadPackageFiles(context.Background(), "core", packagePath, orphanSignature); err == nil {
		t.Fatal("orphan signature unexpectedly accepted")
	}
}

func readMultipart(body io.Reader, boundary string) ([]string, error) {
	reader := multipart.NewReader(body, boundary)
	var result []string
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			return result, nil
		}
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(part)
		if err != nil {
			return nil, err
		}
		result = append(result, part.FormName()+":"+part.FileName()+"="+string(data))
	}
}

func TestParseBaseURLRejectsCredentialBearingURL(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		"https://user:password@example.test",
		"https://example.test?token=secret",
		"https://example.test/#fragment",
		"ftp://example.test",
		"example.test",
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := NewMiko(raw, "key"); err == nil {
				t.Fatalf("NewMiko(%q) unexpectedly succeeded", raw)
			}
		})
	}
}

func TestPublisherRegisterSignerUsesAPIKey(t *testing.T) {
	t.Parallel()
	var gotPath, gotAPIKey, gotBearer, gotContentType, gotBody string
	httpClient := testHTTPClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.EscapedPath()
		gotAPIKey = req.Header.Get("X-API-Key")
		gotBearer = req.Header.Get("Authorization")
		gotContentType = req.Header.Get("Content-Type")
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		gotBody = string(body)
		return testResponse(http.StatusOK, `{"fingerprint":"ABCD"}`), nil
	})
	publisher, err := NewPublisher("https://ayato.example/root", "signer-service-key", WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := publisher.RegisterSigner(context.Background(), []byte("PGP PUBLIC KEY"))
	if err != nil {
		t.Fatal(err)
	}
	if fingerprint != "ABCD" {
		t.Fatalf("fingerprint = %q", fingerprint)
	}
	if gotPath != "/root/api/unstable/auth/signers" || gotAPIKey != "signer-service-key" || gotBearer != "" {
		t.Fatalf("path/auth = %q, X-API-Key %q, Authorization %q", gotPath, gotAPIKey, gotBearer)
	}
	if gotContentType != "application/pgp-keys" || gotBody != "PGP PUBLIC KEY" {
		t.Fatalf("content-type/body = %q, %q", gotContentType, gotBody)
	}
}

func TestPublisherRemoveAllArchitecturesUsesNativeRoute(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath string
	httpClient := testHTTPClient(func(req *http.Request) (*http.Response, error) {
		gotMethod, gotPath = req.Method, req.URL.EscapedPath()
		return testResponse(http.StatusOK, `{}`), nil
	})
	publisher, err := NewPublisher("https://ayato.example/prefix", "publisher-key", WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	if err := publisher.RemovePackageAllArchitectures(context.Background(), "tier/core", "demo/pkg"); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/prefix/api/unstable/repos/tier%2Fcore/packages/demo%2Fpkg" {
		t.Fatalf("method/path = %q %q", gotMethod, gotPath)
	}
}

func TestSignerUsesCommonTransportWithoutMutationRetry(t *testing.T) {
	t.Parallel()
	packagePath := filepath.Join(t.TempDir(), "demo.pkg.tar.zst")
	if err := os.WriteFile(packagePath, []byte("package"), 0o600); err != nil {
		t.Fatal(err)
	}
	var calls int
	httpClient := testHTTPClient(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.Method != http.MethodPost || req.URL.EscapedPath() != "/prefix/api/unstable/sign" {
			t.Fatalf("request = %s %s", req.Method, req.URL.EscapedPath())
		}
		if req.Header.Get("X-API-Key") != "sign-key" || req.Header.Get("Authorization") != "" {
			t.Fatalf("signer credentials = %#v", req.Header)
		}
		return testResponse(http.StatusServiceUnavailable, `{"error":"busy"}`), nil
	})
	signer, err := NewSigner("https://signer.example/prefix", "sign-key", WithHTTPClient(httpClient), WithReadAttempts(3))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := signer.SignFile(context.Background(), packagePath); err == nil {
		t.Fatal("SignFile unexpectedly succeeded")
	}
	if calls != 1 {
		t.Fatalf("sign calls = %d, want exactly one", calls)
	}
}

func TestCatalogClientIsPublicPrefixSafeAndReadRetryable(t *testing.T) {
	t.Parallel()
	var calls int
	httpClient := testHTTPClient(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.EscapedPath() != "/mirror/root/api/unstable/aur/catalog" {
			t.Fatalf("catalog path = %q", req.URL.EscapedPath())
		}
		if hasCredential(req.Header) {
			t.Fatalf("catalog request carried credentials: %#v", req.Header)
		}
		if calls == 1 {
			return testResponse(http.StatusServiceUnavailable, `{"error":"busy"}`), nil
		}
		return testResponse(http.StatusOK, `{"payload":"catalog"}`), nil
	})
	catalog, err := NewCatalog("https://ayato.example/mirror/root", WithHTTPClient(httpClient), WithReadAttempts(2))
	if err != nil {
		t.Fatal(err)
	}
	body, err := catalog.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || string(body) != `{"payload":"catalog"}` {
		t.Fatalf("calls/body = %d, %q", calls, body)
	}
}

func TestCLIStartURLPreservesPrefixAndEncodesQuery(t *testing.T) {
	t.Parallel()
	api, err := NewAyato("https://ayato.example/reverse/proxy", nil)
	if err != nil {
		t.Fatal(err)
	}
	got := api.CLIStartURL(34567, "challenge + / =", "state&value")
	want := "https://ayato.example/reverse/proxy/api/unstable/auth/cli/start?challenge=challenge+%2B+%2F+%3D&port=34567&state=state%26value"
	if got != want {
		t.Fatalf("CLIStartURL = %q, want %q", got, want)
	}
}
