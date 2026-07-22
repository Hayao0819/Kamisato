package initcmd

import (
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/service/source"
)

func Cmd() *cobra.Command {
	targetDir := "."

	var repoName, maintainer, destDir string

	cmd := cobra.Command{
		Use:   "init [dir]",
		Short: "Initialize ayaka repository",
		Long:  "Initializes the Ayaka configuration file.",
		Args:  cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				targetDir = args[0]
			}
			abs, err := source.PrepareTargetDir(targetDir)
			if err != nil {
				return err
			}
			targetDir = abs
			if destDir == "" {
				destDir = filepath.Join(targetDir, "out")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := source.Scaffold(targetDir, repoName, maintainer, destDir)
			if err != nil {
				return err
			}
			cmd.Printf("Initialized Ayaka repository in %s\n", s.TargetDir)
			cmd.Printf("Repository directory: %s\n", s.RepoDir)
			cmd.Printf("Output directory: %s\n", s.DestDir)
			cmd.Printf("Configuration file: %s\n", s.ConfigPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&repoName, "name", "myrepo", "Source repository name")
	cmd.Flags().StringVar(&maintainer, "maintainer", "", "Repository maintainer")
	cmd.Flags().StringVar(&destDir, "dest-dir", "", "Destination directory for built packages (default: <dir>/out)")

	return &cmd
}
