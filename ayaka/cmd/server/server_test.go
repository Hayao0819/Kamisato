package servercmd_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	servercmd "github.com/Hayao0819/Kamisato/ayaka/cmd/server"
)

// TestMain keeps server database writes out of the developer's data directory.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ayaka-server-test-*")
	if err != nil {
		panic(err)
	}
	if err := os.Setenv("XDG_DATA_HOME", dir); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// TestAddRejects3Positionals verifies that the new ExactArgs(1) rejects the
// old three-positional call signature.
func TestAddRejects3Positionals(t *testing.T) {
	cmd := servercmd.Cmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"add", "server1", "user", "pass"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for 3 positionals, got nil")
	}
}

// TestAddPasswordStdin verifies that --password-stdin reads from the reader
// wired by cmd.SetIn rather than from os.Stdin.
func TestAddPasswordStdin(t *testing.T) {
	const server = "test-add-stdin"
	cmd := servercmd.Cmd()
	cmd.SetIn(strings.NewReader("stubpass\n"))
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"add", server, "--username", "testuser", "--password-stdin"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("add --password-stdin: %v", err)
	}

	// Confirm the server appears in a list --json round-trip.
	var out bytes.Buffer
	list := servercmd.Cmd()
	list.SetOut(&out)
	list.SetErr(io.Discard)
	list.SilenceUsage = true
	list.SilenceErrors = true
	list.SetArgs([]string{"list", "--json", server})
	if err := list.Execute(); err != nil {
		t.Fatalf("list after add: %v", err)
	}

	var row map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &row); err != nil {
		t.Fatalf("JSON decode: %v (output: %q)", err, out.String())
	}
	if row["name"] != server {
		t.Errorf("name = %v, want %q", row["name"], server)
	}
	if row["username"] != "testuser" {
		t.Errorf("username = %v, want %q", row["username"], "testuser")
	}
}

func TestAddWithoutTokenPreservesExistingCredential(t *testing.T) {
	const server = "test-add-preserve"
	first := servercmd.Cmd()
	first.SetOut(io.Discard)
	first.SetErr(io.Discard)
	first.SilenceUsage = true
	first.SilenceErrors = true
	first.SetArgs([]string{"add", server, "--token", "existing-token", "--username", "old-name"})
	if err := first.Execute(); err != nil {
		t.Fatal(err)
	}

	update := servercmd.Cmd()
	update.SetOut(io.Discard)
	update.SetErr(io.Discard)
	update.SilenceUsage = true
	update.SilenceErrors = true
	update.SetArgs([]string{"add", server, "--username", "new-name"})
	if err := update.Execute(); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	list := servercmd.Cmd()
	list.SetOut(&out)
	list.SetErr(io.Discard)
	list.SilenceUsage = true
	list.SilenceErrors = true
	list.SetArgs([]string{"list", "--json", "--show-secret", server})
	if err := list.Execute(); err != nil {
		t.Fatal(err)
	}
	var row map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &row); err != nil {
		t.Fatal(err)
	}
	if row["secret"] != "existing-token" || row["username"] != "new-name" {
		t.Fatalf("endpoint update changed credential: %#v", row)
	}
}

func TestAddRejectsEmptyExplicitTokenInput(t *testing.T) {
	cmd := servercmd.Cmd()
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"add", "test-empty-token", "--token-stdin"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("empty --token-stdin unexpectedly succeeded")
	}
}

func TestLoginTokenStdin(t *testing.T) {
	const server = "https://token-stdin.example"
	login := servercmd.Cmd()
	login.SetIn(strings.NewReader("bearer-token\n"))
	login.SetOut(io.Discard)
	login.SetErr(io.Discard)
	login.SilenceUsage = true
	login.SilenceErrors = true
	login.SetArgs([]string{"login", server, "--token-stdin"})
	if err := login.Execute(); err != nil {
		t.Fatalf("login --token-stdin: %v", err)
	}

	var out bytes.Buffer
	list := servercmd.Cmd()
	list.SetOut(&out)
	list.SetErr(io.Discard)
	list.SilenceUsage = true
	list.SilenceErrors = true
	list.SetArgs([]string{"list", "--json", "--show-secret", server})
	if err := list.Execute(); err != nil {
		t.Fatal(err)
	}
	var row map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &row); err != nil {
		t.Fatal(err)
	}
	if row["secret"] != "bearer-token" {
		t.Fatalf("stored token = %v", row["secret"])
	}
}

// TestListSecretOmittedInJSON verifies that --json output does not include the
// "secret" key unless --show-secret is also set.
func TestListSecretOmittedInJSON(t *testing.T) {
	const server = "test-list-no-secret"

	add := servercmd.Cmd()
	add.SetIn(strings.NewReader("s3cr3t\n"))
	add.SetOut(io.Discard)
	add.SetErr(io.Discard)
	add.SilenceUsage = true
	add.SilenceErrors = true
	add.SetArgs([]string{"add", server, "--username", "u", "--password-stdin"})
	if err := add.Execute(); err != nil {
		t.Fatalf("setup add: %v", err)
	}

	var out bytes.Buffer
	list := servercmd.Cmd()
	list.SetOut(&out)
	list.SetErr(io.Discard)
	list.SilenceUsage = true
	list.SilenceErrors = true
	list.SetArgs([]string{"list", "--json", server})
	if err := list.Execute(); err != nil {
		t.Fatalf("list --json: %v", err)
	}

	var row map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &row); err != nil {
		t.Fatalf("JSON decode: %v (output: %q)", err, out.String())
	}
	if _, present := row["secret"]; present {
		t.Errorf("\"secret\" key present in JSON output without --show-secret: %s", out.String())
	}
}

// TestListShowSecretInJSON verifies that --json --show-secret emits the "secret" key.
func TestListShowSecretInJSON(t *testing.T) {
	const server = "test-list-show-secret"

	add := servercmd.Cmd()
	add.SetIn(strings.NewReader("mypass\n"))
	add.SetOut(io.Discard)
	add.SetErr(io.Discard)
	add.SilenceUsage = true
	add.SilenceErrors = true
	add.SetArgs([]string{"add", server, "--username", "u2", "--password-stdin"})
	if err := add.Execute(); err != nil {
		t.Fatalf("setup add: %v", err)
	}

	var out bytes.Buffer
	list := servercmd.Cmd()
	list.SetOut(&out)
	list.SetErr(io.Discard)
	list.SilenceUsage = true
	list.SilenceErrors = true
	list.SetArgs([]string{"list", "--json", "--show-secret", server})
	if err := list.Execute(); err != nil {
		t.Fatalf("list --json --show-secret: %v", err)
	}

	var row map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &row); err != nil {
		t.Fatalf("JSON decode: %v (output: %q)", err, out.String())
	}
	if _, present := row["secret"]; !present {
		t.Errorf("\"secret\" key missing in JSON output with --show-secret: %s", out.String())
	}
}

// TestListNoShorthandS confirms that -s is no longer bound to --show-secret on
// the list subcommand (it is reserved globally for --server).
func TestListNoShorthandS(t *testing.T) {
	cmd := servercmd.Cmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"list", "-s"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("-s should not be a valid flag on server list after removing the shorthand")
	}
}
