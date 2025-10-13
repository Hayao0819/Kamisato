package cmd

import (
	"os"
	"path"
	"path/filepath"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/spf13/cobra"
)

func initCmd() *cobra.Command {

	targetDir := "."
	reponame := "myrepo"
	outDir := ""
	maintainer := "John Smith <john@example.com>"

	cmd := cobra.Command{
		Use:   "init [target directory]",
		Short: "Initialize ayaka repository",
		Long:  "Initializes the Ayaka configuration file.",
		Args:  cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				targetDir = args[0]
			}

			if contents, err := os.ReadDir(targetDir); err != nil {
				if os.IsNotExist(err) {
					if err := os.MkdirAll(targetDir, 0755); err != nil {
						return err
					}
				} else {
					return err
				}
			} else {
				if len(contents) > 0 {
					return &os.PathError{Op: "init", Path: targetDir, Err: os.ErrExist}
				}
			}

			var err error
			targetDir, err = filepath.Abs(targetDir)
			if err != nil {
				return err
			}

			if outDir == "" {
				outDir = path.Join(targetDir, "out")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ayakarcPath := path.Join(targetDir, ".ayakarc.json")
			repoDir := path.Join(targetDir, reponame)

			relRepoDirFromAyakarc, err := filepath.Rel(filepath.Dir(ayakarcPath), repoDir)
			if err != nil {
				cmd.PrintErrf("filepath.Rel(%s, %s) = %s, %s\n", path.Dir(ayakarcPath), repoDir, relRepoDirFromAyakarc, err)
				return err
			}

			relOutDirFromAyakarc, err := filepath.Rel(filepath.Dir(ayakarcPath), outDir)
			if err != nil {
				cmd.PrintErrf("filepath.Rel(%s, %s) = %s, Error(%s)\n", filepath.Dir(ayakarcPath), outDir, relOutDirFromAyakarc, err)
				return err
			}

			ayakarc := conf.AyakaConfig{
				RepoDir: relRepoDirFromAyakarc,
				DestDir: relOutDirFromAyakarc,
				Debug:   false,
			}

			ayakarcBytes, err := ayakarc.Marshal()
			if err != nil {
				return err
			}

			if err := os.WriteFile(ayakarcPath, ayakarcBytes, 0644); err != nil {
				return err
			}

			if err := os.MkdirAll(repoDir, 0755); err != nil {
				return err
			}

			if err := os.MkdirAll(outDir, 0755); err != nil {
				return err
			}

			repoconf := conf.SrcRepoConfig{
				Name:       reponame,
				Maintainer: maintainer,
			}

			repoconfBytes, err := repoconf.Marshal()
			if err != nil {
				return err
			}

			repoconfPath := path.Join(repoDir, "repo.json")
			if err := os.WriteFile(repoconfPath, repoconfBytes, 0644); err != nil {
				return err
			}

			cmd.Printf("Initialized Ayaka repository in %s\n", targetDir)
			cmd.Printf("Repository directory: %s\n", repoDir)
			cmd.Printf("Output directory: %s\n", outDir)
			cmd.Printf("Configuration file: %s\n", ayakarcPath)

			return nil

		},
	}

	return &cmd
}

func init() {
	subCmds = append(subCmds, initCmd())
}
