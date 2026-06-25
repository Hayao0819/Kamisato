package conf

import "testing"

// TestRedteam_TrustAllMixedWithCIDR probes whether a trust-all entry hidden among
// legitimate CIDRs slips past the per-entry switch. Each must be rejected.
func TestRedteam_TrustAllMixedWithCIDR(t *testing.T) {
	for _, bad := range [][]string{
		{"172.16.0.0/12", "0.0.0.0/0"},
		{"0.0.0.0/0", "172.16.0.0/12"},
		{"10.0.0.0/8", "::/0"},
		{"10.0.0.0/8", "*"},
	} {
		c := githubCfg("https://repo.example.com")
		c.Auth.TrustedProxies = bad
		if err := c.Validate(); err == nil {
			t.Fatalf("trusted_proxies %v containing a trust-all entry must be rejected", bad)
		}
	}
}

// TestRedteam_TrustAllSpellingBypass verifies the trust-all rejection is a
// SEMANTIC check (parse each entry as a CIDR and reject prefix length 0), so
// equivalent spellings of the any-net cannot slip past Validate. A purely
// string-based guard over {"*","0.0.0.0/0","::/0"} would miss these and let gin
// trust every peer, re-enabling the spoofed-XFF rate-limit-key attack.
func TestRedteam_TrustAllSpellingBypass(t *testing.T) {
	// Every any-net spelling, plus invalid/whitespace-padded entries, must be
	// rejected (fail-closed config).
	for _, v := range []string{
		"0.0.0.0/0",
		"::/0",
		"0000:0000::/0", // expanded IPv6 any
		"0.0.0.0/00",    // zero-length IPv4 mask
		"not-an-ip",
		" 0.0.0.0/0",
		"0.0.0.0/0 ",
	} {
		c := githubCfg("https://repo.example.com")
		c.Auth.TrustedProxies = []string{v}
		if err := c.Validate(); err == nil {
			t.Fatalf("trusted_proxies %q (any-net or invalid) must be rejected", v)
		}
	}

	// A legitimate fronting-proxy CIDR or single IP is accepted.
	for _, v := range []string{"172.16.0.0/12", "10.0.0.5", "fd00::/8"} {
		c := githubCfg("https://repo.example.com")
		c.Auth.TrustedProxies = []string{v}
		if err := c.Validate(); err != nil {
			t.Fatalf("legitimate trusted_proxies %q must pass: %v", v, err)
		}
	}
}
