package cmd

import "github.com/spf13/cobra"

func aurCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aur",
		Short: "AUR catalog tooling",
	}
	cmd.AddCommand(aurKeygenCmd())
	return cmd
}
