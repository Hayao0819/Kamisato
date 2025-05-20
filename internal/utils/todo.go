package utils

import (
	"errors"

	"github.com/spf13/cobra"
)

func TodoCmd(name string) *cobra.Command {
	cmd := cobra.Command{
		Use:   name,
		Short: "TODO",
		Long:  "TODO",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("this command is not implemented yet")
		},
	}
	return &cmd
}
