package servercmd

import (
	"bufio"
	"fmt"
	"os"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func AddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <server>",
		Short: "Add a server to the local registry",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			username, err := cmd.Flags().GetString("username")
			if err != nil {
				return err
			}
			passwordStdin, err := cmd.Flags().GetBool("password-stdin")
			if err != nil {
				return err
			}

			if username == "" {
				if !term.IsTerminal(int(os.Stdin.Fd())) {
					return fmt.Errorf("stdin is not a terminal; use --username to provide the username")
				}
				fmt.Fprint(cmd.ErrOrStderr(), "Username: ")
				sc := bufio.NewScanner(cmd.InOrStdin())
				if sc.Scan() {
					username = sc.Text()
				}
				if err := sc.Err(); err != nil {
					return err
				}
			}

			var pass string
			if passwordStdin {
				sc := bufio.NewScanner(cmd.InOrStdin())
				if sc.Scan() {
					pass = sc.Text()
				}
				if err := sc.Err(); err != nil {
					return err
				}
			} else if term.IsTerminal(int(os.Stdin.Fd())) {
				pass, err = shared.PromptPassword("Password:")
				if err != nil {
					return err
				}
			} else {
				return fmt.Errorf("stdin is not a terminal; use --password-stdin to provide the password")
			}

			db, err := blinkyutils.ReadServerDB()
			if err != nil {
				return err
			}
			entry := blinkyutils.Server{Username: username}
			if !blinkyutils.StoreSecret(name, pass) {
				entry.Password = pass
			}
			db.Servers[name] = entry
			return blinkyutils.SaveServerDB(db)
		},
	}

	cmd.Flags().String("username", "", "Server username (prompted when omitted on a TTY)")
	cmd.Flags().Bool("password-stdin", false, "Read the password from stdin (one line, docker-style)")

	return cmd
}
