package trustcmd

import (
	"strings"

	"github.com/Hayao0819/Kamisato/internal/errwrap"
	"github.com/Hayao0819/Kamisato/kayo/cmd/shared"
	"github.com/Hayao0819/Kamisato/kayo/gitserve"
	"github.com/Hayao0819/Kamisato/kayo/trust"
	"github.com/spf13/cobra"
)

func trustRemoveCmd() *cobra.Command {
	var maintainer string
	cmd := &cobra.Command{
		Use:   "rm [pkgbase]",
		Short: "Remove a package approval, or a maintainer with --maintainer source/account",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := shared.LoadConfig(cmd)
			if err != nil {
				return err
			}
			store, err := trust.Open(cfg.ResolvedTrustStore())
			if err != nil {
				return err
			}

			switch {
			case maintainer != "":
				source, account, ok := strings.Cut(maintainer, "/")
				if !ok {
					return errwrap.NewErr("--maintainer must be source/account, e.g. aur/jguer")
				}
				store.UntrustMaintainer(source, account)
			case len(args) == 1:
				store.RemoveApproval(args[0])
				_ = gitserve.Remove(cfg.ServedRoot(), args[0])
			default:
				return errwrap.NewErr("specify a pkgbase or --maintainer source/account")
			}
			return store.Save()
		},
	}
	cmd.Flags().StringVar(&maintainer, "maintainer", "", "remove a trusted maintainer (source/account) instead of a package")
	return cmd
}
