package servercmd

import (
	"github.com/spf13/cobra"

	ayatostore "github.com/Hayao0819/Kamisato/internal/api/ayato/auth/store"
)

func RemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "remove <server>",
		Short:             "Remove a server from the local registry",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ayatostore.RemoveEndpoint(args[0])
		},
	}
	return cmd
}
