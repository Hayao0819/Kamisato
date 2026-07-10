package gitcmd

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsPublic(t *testing.T) {
	public := []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "2001:4860:4860::8888"}
	for _, s := range public {
		if !isPublic(net.ParseIP(s)) {
			t.Errorf("isPublic(%q) = false, want true", s)
		}
	}

	// Every class the SSRF guard must reject: loopback, private (v4 + ULA),
	// link-local unicast/multicast, and the unspecified address.
	private := []string{
		"127.0.0.1", "::1",
		"10.1.2.3", "192.168.0.1", "172.16.0.1", "fd00::1",
		"169.254.169.254", "fe80::1",
		"224.0.0.1", "ff02::1",
		"0.0.0.0", "::",
	}
	for _, s := range private {
		if isPublic(net.ParseIP(s)) {
			t.Errorf("isPublic(%q) = true, want false", s)
		}
	}
}

// safeDialContext must refuse to connect when the host resolves only to a
// non-public address, even given as an IP literal (no DNS needed). This is the
// dialer-level half of the DNS-rebinding defense.
func TestSafeDialContextRejectsInternal(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	for _, addr := range []string{"127.0.0.1:80", "169.254.169.254:80", "[::1]:80"} {
		conn, err := pinPublicDial(ctx, "tcp", addr)
		if err == nil {
			_ = conn.Close()
			t.Errorf("safeDialContext(%q) = nil error, want refusal", addr)
		}
	}
}

// TestCloneIntegration performs a real strict https clone through the go-git
// path. It is gated on outbound network availability so CI (offline) skips it.
func TestCloneIntegration(t *testing.T) {
	c, err := net.DialTimeout("tcp", "github.com:443", 3*time.Second)
	if err != nil {
		t.Skipf("no network: %v", err)
	}
	_ = c.Close()

	dir := t.TempDir()
	target := filepath.Join(dir, "repo")
	if err := Clone(context.Background(), CloneOptions{
		URL:    "https://github.com/octocat/Hello-World.git",
		Dir:    target,
		Depth:  1,
		Strict: true,
	}); err != nil {
		t.Fatalf("strict https clone failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, ".git")); err != nil {
		t.Fatalf("clone produced no .git dir: %v", err)
	}
}
