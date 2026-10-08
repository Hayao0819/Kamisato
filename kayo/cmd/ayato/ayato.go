package ayatocmd

import (
	"github.com/spf13/cobra"

	listcmd "github.com/Hayao0819/Kamisato/kayo/cmd/ayato/list"
	pincmd "github.com/Hayao0819/Kamisato/kayo/cmd/ayato/pin"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ayato",
		Short: "Inspect federated ayato sources and their pinned signing keys",
	}
	cmd.AddCommand(listcmd.Cmd(), pincmd.Cmd())
	return cmd
}
