package shared

import (
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/samber/lo"
)

var Config *conf.AyakaConfig

var SrcRepos []*repo.SourceRepo

func InitSrcRepos() error {
	for _, r := range Config.Repos {
		repoconfig, err := conf.LoadSrcRepoConfig(r.Dir)
		if err != nil {
			return utils.WrapErr(err, "failed to load source repository config "+r.Dir)
		}
		sr, err := repo.GetSrcRepo(r.Dir, SrcConfigFromConf(repoconfig))
		if err != nil {
			return utils.WrapErr(err, "failed to load source repository "+r.Dir)
		}
		SrcRepos = append(SrcRepos, sr)
	}
	return nil
}

// SrcConfigFromConf adapts the loaded conf.SrcRepoConfig to the conf-free
// repo.SrcConfig the domain layer consumes.
func SrcConfigFromConf(c *conf.SrcRepoConfig) *repo.SrcConfig {
	if c == nil {
		return nil
	}
	sc := &repo.SrcConfig{
		Name:       c.Name,
		Maintainer: c.Maintainer,
		ArchBuild:  c.ArchBuild,
		Server:     c.Server,
	}
	sc.InstallPkgs.Files = c.InstallPkgs.Files
	sc.InstallPkgs.Names = c.InstallPkgs.Names
	return sc
}

func GetSrcRepo(name string) *repo.SourceRepo {
	if len(SrcRepos) == 0 {
		return nil
	}
	for _, r := range SrcRepos {
		if r.Config.Name == name {
			return r
		}
	}
	return nil
}

func GetDestDir(name string) string {
	for i, r := range SrcRepos {
		if r.Config.Name == name {
			if i < len(Config.Repos) {
				return Config.Repos[i].DestDir
			}
		}
	}
	return ""
}

func GetSrcDir(name string) string {
	for i, r := range SrcRepos {
		if r.Config.Name == name {
			if i < len(Config.Repos) {
				return Config.Repos[i].Dir
			}
		}
	}
	return ""
}

func GetSrcRepoNames() []string {
	return lo.Map(SrcRepos, func(r *repo.SourceRepo, _ int) string {
		return r.Config.Name
	})
}
