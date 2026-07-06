package keycmd

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/cliutil"
)

// keyRow is one line of `key list`: the primary key, then each subkey.
type keyRow struct {
	Kind        string `json:"kind"`
	Fingerprint string `json:"fingerprint"`
	Created     string `json:"created"`
	Expires     string `json:"expires"`
	Status      string `json:"status"`
}

const keyListFormat = "table {{.Kind}}\t{{.Fingerprint}}\t{{.Created}}\t{{.Expires}}\t{{.Status}}"

var keyListHeader = keyRow{Kind: "KIND", Fingerprint: "FINGERPRINT", Created: "CREATED", Expires: "EXPIRES", Status: "STATUS"}

func listCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the signing key and its subkeys",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			k, _, err := shared.LoadSigningKey(cmd)
			if err != nil {
				return err
			}
			primaryStatus := "valid"
			if k.Revoked() {
				primaryStatus = "revoked"
			}
			rows := []keyRow{{
				Kind:        "primary",
				Fingerprint: k.PrimaryFingerprint(),
				Created:     "-",
				Expires:     "-",
				Status:      primaryStatus,
			}}
			for _, s := range k.Subkeys() {
				status := "active"
				switch {
				case s.Revoked:
					status = "revoked"
				case !s.CanSign:
					status = "non-signing"
				case !s.Expires.IsZero() && s.Expires.Before(time.Now()):
					status = "expired"
				}
				expires := "never"
				if !s.Expires.IsZero() {
					expires = s.Expires.Format("2006-01-02")
				}
				rows = append(rows, keyRow{
					Kind:        "subkey",
					Fingerprint: s.Fingerprint,
					Created:     s.Created.Format("2006-01-02"),
					Expires:     expires,
					Status:      status,
				})
			}
			format, err := cliutil.ResolveFormat(cmd, keyListFormat)
			if err != nil {
				return err
			}
			return cliutil.RenderList(cmd.OutOrStdout(), format, keyListHeader, rows)
		},
	}
	cliutil.AddFormatFlags(cmd)
	return cmd
}
