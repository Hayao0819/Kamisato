package servercmd

import (
	"sort"
	"strings"

	blinky_utils "github.com/BrenekH/blinky/cmd/blinky/util"
	"github.com/spf13/cobra"
)

func RemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a server",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			db, err := blinky_utils.ReadServerDB()
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			var completions []string
			for name := range db.Servers {
				if strings.HasPrefix(name, toComplete) {
					completions = append(completions, name)
				}
			}
			sort.Strings(completions)
			return completions, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := blinky_utils.ReadServerDB()
			if err != nil {
				return err
			}
			delete(db.Servers, args[0])
			if db.DefaultServer == args[0] {
				db.DefaultServer = ""
			}
			return blinky_utils.SaveServerDB(db)
		},
	}
	return cmd
}
