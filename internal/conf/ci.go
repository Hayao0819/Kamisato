package conf

import "fmt"

// CIAuthConfig holds the two non-interactive publish credentials a CI pipeline
// can present to ayato's upload route. They are independent: configure either,
// both, or neither (neither = CI publishing stays off, fail-closed).
type CIAuthConfig struct {
	APIKeys    []CIAPIKey   `koanf:"api_keys"`
	GitHubOIDC CIGitHubOIDC `koanf:"github_oidc"`
}

// CIAPIKey is a long-lived shared secret. The CI runner sends it as X-API-Key;
// it may publish only to PublishRepos (ayato/pacman repository names; "*" = any).
type CIAPIKey struct {
	Name         string   `koanf:"name"`
	Key          string   `koanf:"key"`
	PublishRepos []string `koanf:"publish_repos"`
	// Scopes authorize service operations.
	Scopes []string `koanf:"scopes"`
}

// CIGitHubOIDC verifies a GitHub Actions OIDC token (keyless). No secret is
// stored in the workflow repository.
type CIGitHubOIDC struct {
	Enabled bool `koanf:"enabled"`
	// Audience is the fixed aud ayato requires; the workflow must mint its token
	// for exactly this value. A shared/default aud is a confused-deputy risk.
	Audience   string            `koanf:"audience"`
	Publishers []CIOIDCPublisher `koanf:"publishers"`
}

// CIOIDCPublisher allowlists one GitHub source repo. Authorization matches
// RepositoryID (immutable, survives rename) when set, else Repository, and the
// token's ref must be in AllowRefs. PublishRepos is the destination scope ("*" = any).
type CIOIDCPublisher struct {
	Repository   string   `koanf:"repository"`
	RepositoryID string   `koanf:"repository_id"`
	AllowRefs    []string `koanf:"allow_refs"`
	PublishRepos []string `koanf:"publish_repos"`
}

func (c *CIAuthConfig) validate() error {
	names := make(map[string]bool, len(c.APIKeys))
	keys := make(map[string]bool, len(c.APIKeys))
	for i, k := range c.APIKeys {
		if k.Name == "" {
			return fmt.Errorf("auth.ci.api_keys[%d]: name is required", i)
		}
		if k.Key == "" {
			return fmt.Errorf("auth.ci.api_keys[%d]: key is required", i)
		}
		if names[k.Name] {
			return fmt.Errorf("auth.ci.api_keys[%d]: duplicate name %q", i, k.Name)
		}
		if keys[k.Key] {
			return fmt.Errorf("auth.ci.api_keys[%d]: duplicate key", i)
		}
		names[k.Name] = true
		keys[k.Key] = true
		if len(k.PublishRepos) == 0 && len(k.Scopes) == 0 {
			return fmt.Errorf("auth.ci.api_keys[%d]: publish_repos or scopes is required", i)
		}
		for _, scope := range k.Scopes {
			if scope != "*" && scope != "signer:register" {
				return fmt.Errorf("auth.ci.api_keys[%d]: unknown scope %q", i, scope)
			}
		}
	}
	if !c.GitHubOIDC.Enabled {
		return nil
	}
	if c.GitHubOIDC.Audience == "" {
		return fmt.Errorf("auth.ci.github_oidc.audience is required when enabled")
	}
	if len(c.GitHubOIDC.Publishers) == 0 {
		return fmt.Errorf("auth.ci.github_oidc.publishers must not be empty when enabled")
	}
	for i, p := range c.GitHubOIDC.Publishers {
		if p.Repository == "" && p.RepositoryID == "" {
			return fmt.Errorf("auth.ci.github_oidc.publishers[%d]: repository or repository_id is required", i)
		}
		if len(p.AllowRefs) == 0 {
			return fmt.Errorf("auth.ci.github_oidc.publishers[%d]: allow_refs is required", i)
		}
		if len(p.PublishRepos) == 0 {
			return fmt.Errorf("auth.ci.github_oidc.publishers[%d]: publish_repos is required", i)
		}
	}
	return nil
}
