package cmd

import "github.com/spf13/cobra"

func kvCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "kv", Short: "Key-value store maintenance"}
	cmd.AddCommand(auditCmd())
	return cmd
}
