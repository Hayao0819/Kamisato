package removecmd

import (
	"github.com/Hayao0819/Kamisato/ayaka/cmd/server/internal/endpoints"
	ayatostore "github.com/Hayao0819/Kamisato/ayato/client/auth/store"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "remove <server>",
		Short:             "Remove a server from the local registry",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: endpoints.CompleteNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ayatostore.RemoveEndpoint(args[0])
		},
	}
	return cmd
}
