package keyringcmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

func bootstrapCmd() *cobra.Command {
	var (
		repo       string
		keyURL     string
		keyringPkg string
		baseURL    string
		format     string
	)
	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Print the steps a user runs to trust and add this repository",
		Long:  "Emit the pacman-key/pacman-U sequence, with this key's fingerprint filled in, that a user runs to trust the repository before its keyring package can verify. Chicken-and-egg: the keyring package is signed by the very key it distributes, so the key must be trusted out of band first.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			k, _, err := shared.LoadSigningKey(cmd)
			if err != nil {
				return err
			}
			fpr := k.PrimaryFingerprint()
			body := bootstrapText(fpr, repo, keyURL, keyringPkg, baseURL)
			out := cmd.OutOrStdout()
			switch format {
			case "sh", "":
				fmt.Fprint(out, body)
			case "markdown", "md":
				fmt.Fprintf(out, "```sh\n%s```\n", body)
			default:
				return errors.NewErr("unknown --format " + format + " (want sh|markdown)")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "myrepo", "pacman repository name for the pacman.conf snippet")
	cmd.Flags().StringVar(&keyURL, "key-url", "", "URL of the exported public key (for pacman-key --add)")
	cmd.Flags().StringVar(&keyringPkg, "keyring-url", "", "URL of the keyring package (for pacman -U)")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "Repository base URL for the Server line (arch is appended as /$arch)")
	cmd.Flags().StringVar(&format, "format", "sh", "Output format: sh|markdown")
	return cmd
}

func bootstrapText(fpr, repo, keyURL, keyringPkg, baseURL string) string {
	if keyURL == "" {
		keyURL = "<url-to-exported-public-key>"
	}
	if keyringPkg == "" {
		keyringPkg = "<url-to-keyring-package>"
	}
	if baseURL == "" {
		baseURL = "<repository-base-url>"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# 1. Import and locally sign the repository key (trust anchor %s)\n", fpr)
	fmt.Fprintf(&b, "sudo pacman-key --add %s\n", keyURL)
	fmt.Fprintf(&b, "sudo pacman-key --lsign-key %s\n", fpr)
	fmt.Fprintf(&b, "# 2. Install the keyring package directly (the repo is not in pacman.conf yet)\n")
	fmt.Fprintf(&b, "sudo pacman -U %s\n", keyringPkg)
	fmt.Fprintf(&b, "# 3. Add the repository to /etc/pacman.conf:\n")
	fmt.Fprintf(&b, "#   [%s]\n", repo)
	fmt.Fprintf(&b, "#   SigLevel = Required TrustedOnly\n")
	fmt.Fprintf(&b, "#   Server = %s/$arch\n", baseURL)
	fmt.Fprintf(&b, "# 4. Sync:\n")
	fmt.Fprintf(&b, "sudo pacman -Sy\n")
	return b.String()
}
