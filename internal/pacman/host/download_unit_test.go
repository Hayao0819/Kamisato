package host

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetCleanPkgBinaryNeedsNoHostForEmptyInput(t *testing.T) {
	files, cleanup, err := GetCleanPkgBinary(context.Background())
	if err != nil || len(files) != 0 || cleanup != nil {
		t.Fatalf("empty download = %v, %v, %v", files, cleanup, err)
	}
}

func TestGetCleanPkgBinaryRefusesCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	files, cleanup, err := GetCleanPkgBinary(ctx, "test")
	if !errors.Is(err, context.Canceled) || files != nil || cleanup != nil {
		t.Fatalf("canceled download = %v, %v, %v", files, cleanup, err)
	}
}

func TestDownloadNamesCannotBecomePacmanOptions(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$DOWNLOAD_ARGS\"\n"
	if err := os.WriteFile(filepath.Join(dir, "fakeroot"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DOWNLOAD_ARGS", argsFile)
	_, cleanup, err := GetCleanPkgBinary(context.Background(), "--config", "test-pkg")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cleanup.Close(); err != nil {
			t.Error(err)
		}
	}()
	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(args), "pacman\n") || !strings.HasSuffix(string(args), "\n--\n--config\ntest-pkg\n") {
		t.Fatalf("package targets were not separated from pacman options: %s", args)
	}
}
