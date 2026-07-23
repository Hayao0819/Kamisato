package listcmd

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/ayaka/service/report"
	"github.com/Hayao0819/Kamisato/internal/cliutil"
)

func TestListFormatFlagResolution(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{nil, report.DefaultListFormat},
		{[]string{"--json"}, "json"},
		{[]string{"-F", "json"}, "json"},
		{[]string{"-F", "{{.Package}}"}, "{{.Package}}"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			// Create a minimal command just to host the flags.
			cmd := &cobra.Command{}
			cliutil.AddFormatFlags(cmd)
			cmd.SetArgs(tc.args)
			// Parse only; don't execute.
			if err := cmd.ParseFlags(tc.args); err != nil {
				t.Fatalf("ParseFlags: %v", err)
			}
			got, err := cliutil.ResolveFormat(cmd, report.DefaultListFormat)
			if err != nil {
				t.Fatalf("ResolveFormat: %v", err)
			}
			if got != tc.want {
				t.Errorf("ResolveFormat = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestListArgsValidation(t *testing.T) {
	// MaximumNArgs(1): two positional args should fail cobra validation.
	cmd := Cmd()
	cmd.SetArgs([]string{"repo1", "repo2"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for two positional args, got nil")
	}
}
