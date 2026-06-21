package cmd

import (
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/samber/lo"
)

var config *conf.AyakaConfig

var srcRepo []*repo.SourceRepo

// type srcRepoList []*repo.SourceRepo

func initSrcRepos() error {
	for _, r := range config.Repos {
		sr, err := repo.GetSrcRepo(r.Dir)
		if err != nil {
			return utils.WrapErr(err, "failed to load source repository "+r.Dir)
		}
		srcRepo = append(srcRepo, sr)
	}
	return nil
}

func getSrcRepo(name string) *repo.SourceRepo {
	if len(srcRepo) == 0 {
		return nil
	}
	for _, r := range srcRepo {
		if r.Config.Name == name {
			return r
		}
	}
	return nil
}

func getDestDir(name string) string {
	for i, r := range srcRepo {
		if r.Config.Name == name {
			if i < len(config.Repos) {
				return config.Repos[i].DestDir
			}
		}
	}
	return ""
}

func getSrcDir(name string) string {
	for i, r := range srcRepo {
		if r.Config.Name == name {
			if i < len(config.Repos) {
				return config.Repos[i].Dir
			}
		}
	}
	return ""
}

func getSrcRepoNames() []string {
	return lo.Map(srcRepo, func(r *repo.SourceRepo, _ int) string {
		return r.Config.Name
	})
}
