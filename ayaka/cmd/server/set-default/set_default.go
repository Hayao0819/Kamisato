package setdefaultcmd

import (
	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/remote"
	"github.com/Hayao0819/Kamisato/ayaka/cmd/server/internal/endpoints"
	ayatostore "github.com/Hayao0819/Kamisato/ayato/client/auth/store"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "set-default <server>",
		Short:             "Set the default ayato server",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: endpoints.CompleteNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ayatostore.SetDefault(args[0]); err != nil {
				if errors.Is(err, ayatostore.ErrServerNotFound) {
					return errors.WrapErr(remote.ErrServerNotFound, args[0])
				}
				return err
			}
			return nil
		},
	}
	return cmd
}
