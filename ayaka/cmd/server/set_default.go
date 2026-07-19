package servercmd

import (
	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/serverstore"
)

func SetDefaultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "set-default <server>",
		Short:             "Set the default ayato server",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := serverstore.SetDefault(args[0]); err != nil {
				if errors.Is(err, serverstore.ErrServerNotFound) {
					return errors.WrapErr(shared.ErrServerNotFound, args[0])
				}
				return err
			}
			return nil
		},
	}
	return cmd
}
