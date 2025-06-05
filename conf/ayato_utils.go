package conf

import "path"

func (c *AyatoConfig) DbPath() string {
	return path.Join(c.Store.BadgerDB, "kv-db")
}

func (c *AyatoConfig) RepoNames() []string {
	repoNames := make([]string, len(c.Repos))
	for i, repo := range c.Repos {
		repoNames[i] = repo.Name
	}
	return repoNames
}

func (c *AyatoConfig) Repo(name string) *BinRepoConfig {
	for _, repo := range c.Repos {
		if repo.Name == name {
			return &repo
		}
	}
	return nil
}
