package repocmd

import (
	gccmd "github.com/Hayao0819/Kamisato/ayato/cmd/repo/gc"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{Use: "repo", Short: "Repository maintenance"}
	cmd.AddCommand(gccmd.Cmd())
	return cmd
}
