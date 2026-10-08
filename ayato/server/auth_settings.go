package server

import (
	"github.com/Hayao0819/Kamisato/ayato/auth"
	"github.com/Hayao0819/Kamisato/ayato/config"
)

func ciSettings(value config.CIAuthConfig) auth.CISettings {
	settings := auth.CISettings{
		APIKeys: make([]auth.CIAPIKey, len(value.APIKeys)),
		GitHubOIDC: auth.CIGitHubOIDC{
			Enabled:  value.GitHubOIDC.Enabled,
			Audience: value.GitHubOIDC.Audience,
		},
	}
	for index, key := range value.APIKeys {
		settings.APIKeys[index] = auth.CIAPIKey{
			Name:         key.Name,
			Key:          key.Key,
			PublishRepos: key.PublishRepos,
			Scopes:       key.Scopes,
		}
	}
	settings.GitHubOIDC.Publishers = make([]auth.CIOIDCPublisher, len(value.GitHubOIDC.Publishers))
	for index, publisher := range value.GitHubOIDC.Publishers {
		settings.GitHubOIDC.Publishers[index] = auth.CIOIDCPublisher{
			Repository:   publisher.Repository,
			RepositoryID: publisher.RepositoryID,
			AllowRefs:    publisher.AllowRefs,
			PublishRepos: publisher.PublishRepos,
		}
	}
	return settings
}
