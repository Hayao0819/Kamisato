package oauth

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/internal/buildclient"
)

func fixedRequester(dc buildclient.DeviceCodeResponse) DeviceRequester {
	return func(context.Context, string) (buildclient.DeviceCodeResponse, error) { return dc, nil }
}

// noSleep makes the polling loop spin without real delay.
func noSleep(context.Context, time.Duration) error { return nil }

// scriptedPoller returns each result in turn, then the last one forever.
func scriptedPoller(results ...buildclient.DeviceTokenResult) (DevicePoller, *int) {
	calls := 0
	return func(context.Context, string, string) (buildclient.DeviceTokenResult, error) {
		r := results[min(calls, len(results)-1)]
		calls++
		return r, nil
	}, &calls
}

func TestDeviceLoginApproved(t *testing.T) {
	dc := buildclient.DeviceCodeResponse{
		DeviceCode: "dc", UserCode: "BCDF-GHJK",
		VerificationURI: "https://ayato.example/api/unstable/auth/device", ExpiresIn: 600, Interval: 5,
	}
	poll, calls := scriptedPoller(
		buildclient.DeviceTokenResult{Status: "authorization_pending"},
		buildclient.DeviceTokenResult{Status: "authorization_pending"},
		buildclient.DeviceTokenResult{Token: "the-token", Refresh: "the-refresh", Login: "octocat", ID: 7},
	)

	var out strings.Builder
	token, refresh, login, err := DeviceLogin(context.Background(), "https://ayato.example",
		WithDeviceOutput(&out),
		WithDeviceRequester(fixedRequester(dc)),
		WithDevicePoller(poll),
		WithDeviceSleep(noSleep),
	)
	if err != nil {
		t.Fatal(err)
	}
	if token != "the-token" || refresh != "the-refresh" || login != "octocat" {
		t.Fatalf("got token=%q refresh=%q login=%q", token, refresh, login)
	}
	if *calls != 3 {
		t.Fatalf("polled %d times, want 3 (two pending, one success)", *calls)
	}
	// The instructions must surface the code and the URL for the user.
	if s := out.String(); !strings.Contains(s, "BCDF-GHJK") || !strings.Contains(s, dc.VerificationURI) {
		t.Fatalf("instructions %q must contain the user code and URL", s)
	}
}

func TestDeviceLoginSlowDownThenApproved(t *testing.T) {
	poll, calls := scriptedPoller(
		buildclient.DeviceTokenResult{Status: "slow_down"},
		buildclient.DeviceTokenResult{Token: "t", Login: "u"},
	)
	token, _, _, err := DeviceLogin(context.Background(), "https://x",
		WithDeviceOutput(io.Discard),
		WithDeviceRequester(fixedRequester(buildclient.DeviceCodeResponse{DeviceCode: "dc", Interval: 1, ExpiresIn: 600})),
		WithDevicePoller(poll),
		WithDeviceSleep(noSleep),
	)
	if err != nil || token != "t" {
		t.Fatalf("slow_down then approve: token=%q err=%v", token, err)
	}
	if *calls != 2 {
		t.Fatalf("polled %d times, want 2", *calls)
	}
}

func TestDeviceLoginDeniedAndExpired(t *testing.T) {
	cases := map[string]string{
		"access_denied": "denied",
		"expired_token": "expired",
	}
	for status, want := range cases {
		poll, _ := scriptedPoller(buildclient.DeviceTokenResult{Status: status})
		_, _, _, err := DeviceLogin(context.Background(), "https://x",
			WithDeviceOutput(io.Discard),
			WithDeviceRequester(fixedRequester(buildclient.DeviceCodeResponse{DeviceCode: "dc", Interval: 1, ExpiresIn: 600})),
			WithDevicePoller(poll),
			WithDeviceSleep(noSleep),
		)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("status %q: err = %v, want to mention %q", status, err, want)
		}
	}
}
