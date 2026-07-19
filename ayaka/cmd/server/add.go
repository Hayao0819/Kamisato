package servercmd

import (
	"bufio"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/serverstore"
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
			server := args[0]

			username, err := cmd.Flags().GetString("username")
			if err != nil {
				return err
			}
			token, err := cmd.Flags().GetString("token")
			if err != nil {
				return err
			}
			tokenStdin, err := cmd.Flags().GetBool("token-stdin")
			if err != nil {
				return err
			}
			passwordStdin, err := cmd.Flags().GetBool("password-stdin")
			if err != nil {
				return err
			}
			clearCredentials, err := cmd.Flags().GetBool("clear-credentials")
			if err != nil {
				return err
			}
			if token != "" && (tokenStdin || passwordStdin) {
				return fmt.Errorf("--token and --token-stdin cannot be used together")
			}
			if clearCredentials && (cmd.Flags().Changed("token") || tokenStdin || passwordStdin) {
				return fmt.Errorf("--clear-credentials cannot be combined with token input")
			}

			if tokenStdin || passwordStdin {
				sc := bufio.NewScanner(cmd.InOrStdin())
				if sc.Scan() {
					token = sc.Text()
				}
				if err := sc.Err(); err != nil {
					return err
				}
			}
			credentialInput := cmd.Flags().Changed("token") || tokenStdin || passwordStdin
			if credentialInput && token == "" {
				return fmt.Errorf("Bearer token input is empty; omit the token option to preserve credentials or use --clear-credentials to remove them")
			}

			if clearCredentials {
				return serverstore.SaveStaticToken(server, username, "")
			}
			if !credentialInput {
				return serverstore.SaveEndpoint(server, username)
			}
			return serverstore.SaveStaticToken(server, username, token)
		},
	}

	cmd.Flags().String("token", "", "Static Ayato Bearer token (prefer 'server login' for OAuth)")
	cmd.Flags().Bool("token-stdin", false, "Read the Bearer token from stdin (one line)")
	cmd.Flags().String("username", "", "Optional display name associated with the token")
	cmd.Flags().Bool("password-stdin", false, "Deprecated alias for --token-stdin")
	cmd.Flags().Bool("clear-credentials", false, "Remove stored access and refresh tokens while keeping the endpoint")
	_ = cmd.Flags().MarkDeprecated("password-stdin", "use --token-stdin; native Ayato authentication uses Bearer tokens")

	return cmd
}
