package kvcmd

import (
	auditcmd "github.com/Hayao0819/Kamisato/ayato/cmd/kv/audit"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{Use: "kv", Short: "Key-value store maintenance"}
	cmd.AddCommand(auditcmd.Cmd())
	return cmd
}
