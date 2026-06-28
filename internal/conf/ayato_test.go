package conf

import "testing"

const testSessionSecret = "0123456789abcdef0123456789abcdef" // 32 bytes

// githubCfg builds a fully valid GitHub-enabled config so each test can perturb one
// field at a time.
func githubCfg(origin string) *AyatoConfig {
	c := &AyatoConfig{}
	c.Auth.GitHub.ClientID = "cid"
	c.Auth.GitHub.ClientSecret = "secret"
	c.Auth.PublicOrigin = origin
	c.Auth.SessionSecret = []string{testSessionSecret}
	c.Auth.TrustedProxies = []string{"172.16.0.0/12"}
	return c
}

func TestValidateNoGitHubIsOK(t *testing.T) {
	if err := (&AyatoConfig{}).Validate(); err != nil {
		t.Fatalf("no GitHub config: Validate = %v, want nil", err)
	}
}

func TestValidateGitHubRequiresPublicOrigin(t *testing.T) {
	if err := githubCfg("").Validate(); err == nil {
		t.Fatal("GitHub enabled without public_origin must fail")
	}
	if err := githubCfg("https://repo.example.com").Validate(); err != nil {
		t.Fatalf("GitHub with valid public_origin: Validate = %v, want nil", err)
	}
}

func TestValidateGitHubRequiresBothCredentials(t *testing.T) {
	c := &AyatoConfig{}
	c.Auth.GitHub.ClientID = "cid"
	c.Auth.PublicOrigin = "https://repo.example.com"
	if err := c.Validate(); err == nil {
		t.Fatal("client_id without client_secret must fail")
	}
}

func TestValidateRejectsBadPublicOrigin(t *testing.T) {
	for _, bad := range []string{
		"repo.example.com",             // no scheme
		"ftp://repo.example.com",       // wrong scheme
		"https://",                     // no host
		"https://repo.example.com/sub", // has a path
	} {
		if err := githubCfg(bad).Validate(); err == nil {
			t.Fatalf("public_origin %q must be rejected", bad)
		}
	}
}

func TestValidateRequiresSessionSecret(t *testing.T) {
	c := githubCfg("https://repo.example.com")
	c.Auth.SessionSecret = nil
	if err := c.Validate(); err == nil {
		t.Fatal("GitHub enabled without session_secret must fail")
	}

	c.Auth.SessionSecret = []string{"too-short"}
	if err := c.Validate(); err == nil {
		t.Fatal("session_secret under 32 bytes must fail")
	}

	// At least one key of >= 32 bytes is enough (rotation: the others may be old).
	c.Auth.SessionSecret = []string{"too-short", testSessionSecret}
	if err := c.Validate(); err != nil {
		t.Fatalf("one usable session_secret must pass: %v", err)
	}
}

func TestValidateRequiresTrustedProxies(t *testing.T) {
	c := githubCfg("https://repo.example.com")
	c.Auth.TrustedProxies = nil
	if err := c.Validate(); err == nil {
		t.Fatal("GitHub enabled without trusted_proxies must fail")
	}

	for _, trustAll := range []string{"*", "0.0.0.0/0", "::/0"} {
		c.Auth.TrustedProxies = []string{trustAll}
		if err := c.Validate(); err == nil {
			t.Fatalf("trust-all trusted_proxies %q must be rejected", trustAll)
		}
	}
}
