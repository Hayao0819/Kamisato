package removecmd

import (
	"strings"

	"github.com/spf13/cobra"

	cmdline "github.com/Hayao0819/Kamisato/internal/cli"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/kayo/cmd/internal/settings"
	"github.com/Hayao0819/Kamisato/kayo/gitserve"
	"github.com/Hayao0819/Kamisato/kayo/trust"
)

func Cmd() *cobra.Command {
	var maintainer string
	cmd := &cobra.Command{
		Use:     "remove [<pkgname>]",
		Aliases: []string{"rm"},
		Short:   "Remove a package approval, or a maintainer with --maintainer source/account",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
				return &cmdline.UsageError{Err: err}
			}
			if cmd.Flags().Changed("maintainer") {
				if len(args) != 0 {
					return &cmdline.UsageError{Err: errors.NewErr("specify a pkgbase or --maintainer, not both")}
				}
				source, account, ok := strings.Cut(maintainer, "/")
				if !ok || source == "" || account == "" || strings.Contains(account, "/") {
					return &cmdline.UsageError{Err: errors.NewErr("--maintainer must be source/account, e.g. aur/jguer")}
				}
			} else if len(args) != 1 {
				return &cmdline.UsageError{Err: errors.NewErr("specify a pkgbase or --maintainer source/account")}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := settings.Load(cmd.Flags())
			if err != nil {
				return err
			}
			store, err := trust.Open(cfg.ResolvedTrustStore())
			if err != nil {
				return err
			}

			switch {
			case maintainer != "":
				source, account, _ := strings.Cut(maintainer, "/")
				store.UntrustMaintainer(source, account)
			case len(args) == 1:
				if err := gitserve.Remove(cfg.ServedRoot(), args[0]); err != nil {
					return err
				}
				store.RemoveApproval(args[0])
			}
			return store.Save()
		},
	}
	cmd.Flags().StringVar(&maintainer, "maintainer", "", "remove a trusted maintainer (source/account) instead of a package")
	return cmd
}
