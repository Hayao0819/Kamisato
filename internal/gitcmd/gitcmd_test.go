package gitcmd

import "testing"

func TestValidateRemote(t *testing.T) {
	// Allowed forms (IP literals avoid DNS in tests).
	allow := []string{
		"https://8.8.8.8/repo.git",
		"git://8.8.8.8/repo.git",
		"ssh://git@8.8.8.8/repo.git",
		"git@github.com:user/repo.git", // scp-like ssh
	}
	for _, u := range allow {
		if err := ValidateRemote(u); err != nil {
			t.Errorf("ValidateRemote(%q) = %v, want allowed", u, err)
		}
	}

	// Rejected: SSRF to internal hosts, plaintext http, local/file, and the
	// ext:: transport-helper RCE.
	reject := []string{
		"file:///etc/passwd",
		"ext::sh -c id",
		"ext::sh -c 'touch /tmp/pwned'",
		"http://8.8.8.8/x",
		"/local/path/repo",
		"https://127.0.0.1/x",
		"https://169.254.169.254/latest/meta-data", // cloud metadata
		"https://10.1.2.3/x",
		"git://192.168.0.1/x",
	}
	for _, u := range reject {
		if err := ValidateRemote(u); err == nil {
			t.Errorf("ValidateRemote(%q) = nil, want rejected", u)
		}
	}
}
