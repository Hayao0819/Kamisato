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

// TestRedteam_TrustAllSpellingBypass verifies the trust-all rejection is semantic
// (parse each entry, reject prefix length 0) so any-net spellings can't slip past
// Validate — a string-only guard would let gin trust every peer and re-enable the
// spoofed-XFF rate-limit attack.
func TestRedteam_TrustAllSpellingBypass(t *testing.T) {
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

	for _, v := range []string{"172.16.0.0/12", "10.0.0.5", "fd00::/8"} {
		c := githubCfg("https://repo.example.com")
		c.Auth.TrustedProxies = []string{v}
		if err := c.Validate(); err != nil {
			t.Fatalf("legitimate trusted_proxies %q must pass: %v", v, err)
		}
	}
}
