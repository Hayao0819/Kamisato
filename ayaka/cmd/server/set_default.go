package servercmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cli"
	ayatostore "github.com/Hayao0819/Kamisato/internal/api/ayato/auth/store"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

func SetDefaultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "set-default <server>",
		Short:             "Set the default ayato server",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ayatostore.SetDefault(args[0]); err != nil {
				if errors.Is(err, ayatostore.ErrServerNotFound) {
					return errors.WrapErr(cli.ErrServerNotFound, args[0])
				}
				return err
			}
			return nil
		},
	}
	return cmd
}
