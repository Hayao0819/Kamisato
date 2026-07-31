package cmd

import "github.com/spf13/cobra"

func repoCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "repo", Short: "Repository maintenance"}
	cmd.AddCommand(gcCmd())
	return cmd
}
