package mikocmd

import (
	"testing"

	cancelcmd "github.com/Hayao0819/Kamisato/ayaka/cmd/miko/cancel"
	logscmd "github.com/Hayao0819/Kamisato/ayaka/cmd/miko/logs"
	"github.com/spf13/cobra"
)

func TestStreamCommandsRequireOneJobID(t *testing.T) {
	commands := []struct {
		name string
		new  func() *cobra.Command
	}{
		{name: "logs", new: logscmd.Cmd},
		{name: "cancel", new: cancelcmd.Cmd},
	}

	for _, command := range commands {
		t.Run(command.name, func(t *testing.T) {
			cmd := command.new()
			if err := cmd.Args(cmd, []string{"job-123"}); err != nil {
				t.Fatalf("one job ID rejected: %v", err)
			}
			for _, args := range [][]string{nil, {"job-1", "job-2"}} {
				if err := cmd.Args(cmd, args); err == nil {
					t.Errorf("%d arguments accepted, want exactly one", len(args))
				}
			}
		})
	}
}
