package service

import (
	"net/http"
	"testing"
	"time"
)

func TestUpstreamRetryAfterCapsBeforeDurationConversion(t *testing.T) {
	resp := &http.Response{Header: http.Header{"Retry-After": []string{"9223372036854775807"}}}
	if got := upstreamRetryAfter(resp, 0); got != 30*time.Second {
		t.Fatalf("retry delay = %s, want 30s", got)
	}
}
