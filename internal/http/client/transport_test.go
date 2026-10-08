package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func TestAuthenticatedRequestOwnsOriginAndContext(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(t.Context(), key{}, "caller")
	source := BearerTokenSourceFunc(func(got context.Context) (string, error) {
		if got.Value(key{}) != "caller" {
			t.Fatal("token source lost the request context")
		}
		return "token", nil
	})
	requester, err := NewBearerRequester("https://api.example", source)
	if err != nil {
		t.Fatal(err)
	}
	transport := requester.Transport()
	target, _ := url.Parse("https://api.example:443/packages")
	req, err := transport.NewRequest(ctx, http.MethodGet, target, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("Authorization") != "Bearer token" || req.Context().Value(key{}) != "caller" {
		t.Fatalf("request credentials/context lost: %+v", req)
	}
	other, _ := url.Parse("https://other.example/packages")
	if _, err := transport.NewRequest(ctx, http.MethodGet, other, nil, true); err == nil {
		t.Fatal("credentials accepted for another origin")
	}
	if _, err := NewBearerRequester("http://api.example", source); err == nil {
		t.Fatal("bearer credentials accepted over cleartext")
	}
}

func TestEndpointEscapesEachPathSegment(t *testing.T) {
	base, err := ParseBaseURL("https://api.example/api/")
	if err != nil {
		t.Fatal(err)
	}
	target := EndpointURL(base, "packages", "repo/name?x#y")
	if target.String() != "https://api.example/api/packages/repo%2Fname%3Fx%23y" {
		t.Fatalf("endpoint = %s", target)
	}
	if base.String() != "https://api.example/api" {
		t.Fatal("endpoint construction mutated the base URL")
	}
}

func TestRedirectPolicyDoesNotReplayCredentials(t *testing.T) {
	for _, authenticated := range []bool{true, false} {
		t.Run(map[bool]string{true: "authenticated", false: "public"}[authenticated], func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					if authenticated && req.Header.Get("Authorization") != "Bearer token" {
						t.Error("first request lost authentication")
					}
					resp := response(http.StatusFound, "")
					resp.Header.Set("Location", "https://download.example/archive")
					return resp, nil
				}
				if hasCredential(req.Header) {
					t.Fatal("redirect exposed credentials")
				}
				return response(http.StatusOK, "archive"), nil
			})}
			transport, err := newBearerTransport("https://api.example", StaticBearer("token"), WithHTTPClient(client))
			if err != nil {
				t.Fatal(err)
			}
			resp, err := transport.Get(t.Context(), transport.Endpoint("archive"), authenticated)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			wantStatus, wantCalls := http.StatusOK, 2
			if authenticated {
				wantStatus, wantCalls = http.StatusFound, 1
			}
			if resp.StatusCode != wantStatus || calls != wantCalls {
				t.Fatalf("status/calls = %d/%d, want %d/%d", resp.StatusCode, calls, wantStatus, wantCalls)
			}
		})
	}
}

func TestRetryRequiresExplicitReplaySafePolicy(t *testing.T) {
	for _, policy := range []RetryPolicy{NoRetry, RetryReplaySafe} {
		t.Run(map[RetryPolicy]string{NoRetry: "mutation", RetryReplaySafe: "replay-safe"}[policy], func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				body, err := io.ReadAll(req.Body)
				if err != nil || string(body) != `{"name":"package"}` {
					t.Fatalf("attempt %d body = %q, error %v", calls, body, err)
				}
				if calls == 1 {
					return response(http.StatusBadGateway, `{"error":"temporary"}`), nil
				}
				return response(http.StatusOK, `{"ok":true}`), nil
			})}
			transport, err := NewPublicTransport("https://api.example", WithHTTPClient(client), WithReadAttempts(2))
			if err != nil {
				t.Fatal(err)
			}
			var result struct{ OK bool }
			err = transport.DoJSON(t.Context(), policy, http.MethodPost, transport.Endpoint("packages"), false,
				map[string]string{"name": "package"}, &result, http.StatusOK, "test request")
			if policy == NoRetry {
				var responseErr *ResponseError
				if calls != 1 || !errors.As(err, &responseErr) || responseErr.StatusCode != http.StatusBadGateway {
					t.Fatalf("mutation was retried or lost response classification: calls=%d error=%v", calls, err)
				}
			} else if calls != 2 || err != nil || !result.OK {
				t.Fatalf("replay-safe retry: calls=%d result=%+v error=%v", calls, result, err)
			}
		})
	}
}

func TestReadBytesBoundsBothKnownAndStreamingLengths(t *testing.T) {
	for _, contentLength := range []int64{5, -1} {
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			resp := response(http.StatusOK, "large")
			resp.ContentLength = contentLength
			return resp, nil
		})}
		transport, err := NewPublicTransport("https://api.example", WithHTTPClient(client))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := transport.ReadBytes(t.Context(), transport.Endpoint("archive"), 4, "read archive", ""); err == nil {
			t.Fatalf("length %d accepted an oversized response", contentLength)
		}
	}
}

func TestCanceledRequestDoesNotRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return nil, req.Context().Err()
	})}
	transport, err := NewPublicTransport("https://api.example", WithHTTPClient(client))
	if err != nil {
		t.Fatal(err)
	}
	err = transport.DoJSON(ctx, RetryReplaySafe, http.MethodGet, transport.Endpoint("packages"), false, nil, nil, http.StatusOK, "read packages")
	if !errors.Is(err, context.Canceled) || calls > 1 {
		t.Fatalf("canceled request: calls=%d error=%v", calls, err)
	}
}
