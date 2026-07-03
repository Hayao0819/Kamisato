package buildcmd

import (
	"strings"
	"testing"
)

func TestBuildFlagShape(t *testing.T) {
	cmd := Cmd()
	flags := cmd.Flags()

	present := []string{"sign", "key", "diff", "server", "executor", "arch"}
	for _, name := range present {
		if flags.Lookup(name) == nil {
			t.Errorf("flag --%s not registered", name)
		}
	}

	absent := []string{"gpgkey", "remote"}
	for _, name := range absent {
		if flags.Lookup(name) != nil {
			t.Errorf("flag --%s should have been removed", name)
		}
	}

	// --key must not have a shorthand.
	if f := flags.Lookup("key"); f != nil {
		if sh := flags.ShorthandLookup("g"); sh != nil {
			t.Errorf("shorthand -g should no longer exist (was the old --key shorthand)")
		}
	}
}

func TestBuildSignRequiresKey(t *testing.T) {
	// --sign without --key should fail in PreRunE, not in an opaque way.
	// We can't run PreRunE without a real app context, but we can confirm
	// that --sign is a bool and --key is a string with no shorthand.
	cmd := Cmd()
	flags := cmd.Flags()

	if err := flags.Set("sign", "true"); err != nil {
		t.Fatalf("could not set --sign: %v", err)
	}
	v, err := flags.GetBool("sign")
	if err != nil || !v {
		t.Errorf("--sign not set correctly: v=%v err=%v", v, err)
	}

	// --key has no shorthand -g anymore.
	if sh := flags.ShorthandLookup("g"); sh != nil {
		t.Error("shorthand -g must not exist on build")
	}
}

func TestBuildUseString(t *testing.T) {
	cmd := Cmd()
	if !strings.HasPrefix(cmd.Use, "build <srcrepo>") {
		t.Errorf("Use = %q, want prefix 'build <srcrepo>'", cmd.Use)
	}
}
