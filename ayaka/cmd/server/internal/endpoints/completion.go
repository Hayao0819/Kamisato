package endpoints

import (
	"github.com/Hayao0819/Kamisato/ayato/client/auth/store"
	"github.com/spf13/cobra"
)

func CompleteNames(_ *cobra.Command, _ []string, prefix string) ([]string, cobra.ShellCompDirective) {
	return store.Names(prefix), cobra.ShellCompDirectiveNoFileComp
}
