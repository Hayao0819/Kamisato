package cmd

import (
	"path"

	"github.com/Hayao0819/Kamisato/ayaka/abs"
	"github.com/Hayao0819/Kamisato/conf"
	"github.com/Hayao0819/Kamisato/repo"
	"github.com/spf13/cobra"
)

func buildCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "build",
		Short: "Build packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := repo.GetRepository(config.RepoDir)
			if err != nil {
				return err
			}
			builder := abs.Target{
				Arch: "x86_64",
			}

			ac, err := conf.LoadAyakaConfig()
			if err != nil {
				return err
			}

			outDir := path.Join(ac.DestDir, repo.Config.Name)

			return repo.Build(&builder, outDir, args...)
		},
	}

	return &cmd
}

func init() {
	subCmds.Add(buildCmd())
}
