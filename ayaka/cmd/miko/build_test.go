package mikocmd

import (
	"strings"
	"testing"
	"time"
)

func TestDurationToMinutes(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want int
	}{
		{0, 0},
		{-time.Second, 0},
		{time.Second, 1},
		{30 * time.Second, 1},
		{59 * time.Second, 1},
		{time.Minute, 1},
		{90 * time.Second, 2},
		{30 * time.Minute, 30},
		{2 * time.Hour, 120},
	}
	for _, tc := range cases {
		got := durationToMinutes(tc.in)
		if got != tc.want {
			t.Errorf("durationToMinutes(%v) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestMikoBuildFlagShape(t *testing.T) {
	cmd := mikoBuildCmd()
	flags := cmd.Flags()

	present := []string{"sign-local", "key", "passphrase-file", "git", "ref", "subdir", "arch", "timeout"}
	for _, name := range present {
		if flags.Lookup(name) == nil {
			t.Errorf("flag --%s not registered", name)
		}
	}

	absent := []string{"local-key", "local-pass"}
	for _, name := range absent {
		if flags.Lookup(name) != nil {
			t.Errorf("flag --%s should have been removed", name)
		}
	}
}

func TestMikoBuildTimeoutIsDuration(t *testing.T) {
	cmd := mikoBuildCmd()
	if err := cmd.Flags().Set("timeout", "30m"); err != nil {
		t.Fatalf("could not set --timeout to 30m: %v", err)
	}
	d, err := cmd.Flags().GetDuration("timeout")
	if err != nil {
		t.Fatalf("GetDuration: %v", err)
	}
	if d != 30*time.Minute {
		t.Errorf("--timeout 30m parsed as %v, want 30m", d)
	}
}

func TestMikoBuildRequiredTogether(t *testing.T) {
	// --sign-local without --key must error due to MarkFlagsRequiredTogether.
	// Pass --git to bypass the PreRunE local-repo check so cobra reaches the
	// flag-group validation, which fires after PreRunE.
	cmd := mikoBuildCmd()
	cmd.SetArgs([]string{"--sign-local", "--git", "https://aur.archlinux.org/foo.git", "myrepo"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --sign-local is set without --key")
	}
	if !strings.Contains(err.Error(), "key") {
		t.Errorf("error %q does not mention 'key'", err.Error())
	}
}
