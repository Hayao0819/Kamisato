package addcmd

import (
	"fmt"
	"time"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/internal/keyinput"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	var expire time.Duration
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new signing subkey",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			key, passphrase, err := keyinput.DefaultProvider().Load(keyinput.Read(cmd))
			if err != nil {
				return err
			}
			if err := key.AddSubkey(expire, passphrase); err != nil {
				return errors.WrapErr(err, "failed to add subkey")
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Added a new signing subkey. Republish the keyring so it reaches users.")
			return nil
		},
	}
	cmd.Flags().DurationVar(&expire, "expire", 365*24*time.Hour, "Subkey validity; 0 = never")
	return cmd
}
