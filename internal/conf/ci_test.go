package conf

import "testing"

func TestCIAPIKeyValidation(t *testing.T) {
	valid := CIAPIKey{Name: "signer", Key: "secret", Scopes: []string{"signer:register"}}
	if err := (&CIAuthConfig{APIKeys: []CIAPIKey{valid}}).validate(); err != nil {
		t.Fatalf("valid scoped key rejected: %v", err)
	}
	for _, tc := range []struct {
		name string
		keys []CIAPIKey
	}{
		{name: "missing grant", keys: []CIAPIKey{{Name: "empty", Key: "secret"}}},
		{name: "unknown scope", keys: []CIAPIKey{{Name: "bad", Key: "secret", Scopes: []string{"root"}}}},
		{name: "duplicate name", keys: []CIAPIKey{valid, {Name: valid.Name, Key: "other", PublishRepos: []string{"core"}}}},
		{name: "duplicate secret", keys: []CIAPIKey{valid, {Name: "other", Key: valid.Key, PublishRepos: []string{"core"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := (&CIAuthConfig{APIKeys: tc.keys}).validate(); err == nil {
				t.Fatal("validation unexpectedly succeeded")
			}
		})
	}
}
