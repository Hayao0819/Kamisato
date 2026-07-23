package bumpcmd

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

type recordingBumper struct {
	names   []string
	by      string
	message string
	commits int
}

func (r *recordingBumper) Bump(src *repo.SourceRepo, names []string, by string, _ io.Writer) ([]*pkg.SourcePackage, error) {
	r.names = names
	r.by = by
	return src.Pkgs, nil
}

func (r *recordingBumper) Commit(_ string, _ []*pkg.SourcePackage, message string) (string, error) {
	r.message = message
	r.commits++
	return "deadbeef", nil
}

func testApp(t *testing.T) *app.App {
	t.Helper()
	dir := t.TempDir()
	srcinfo := "pkgbase = foo\n\tpkgver = 1.0\n\tpkgrel = 1\n\tarch = any\n\npkgname = foo\n"
	if err := os.WriteFile(filepath.Join(dir, ".SRCINFO"), []byte(srcinfo), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := pkg.OpenSourcePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	return &app.App{SrcRepos: []*repo.SourceRepo{{
		Config: &repo.SrcConfig{Name: "test"},
		Pkgs:   []*pkg.SourcePackage{p},
		Dir:    dir,
	}}}
}

func TestBumpFlagsReachService(t *testing.T) {
	rec := &recordingBumper{}
	cmd := newCommand(rec)
	cmd.SetContext(app.WithContext(t.Context(), testApp(t)))
	cmd.SetArgs([]string{"test", "foo", "--by", "1", "--message", "msg"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if rec.by != "1" || len(rec.names) != 1 || rec.names[0] != "foo" {
		t.Errorf("service got names=%v by=%q", rec.names, rec.by)
	}
	if rec.message != "msg" || rec.commits != 1 {
		t.Errorf("commit message=%q commits=%d", rec.message, rec.commits)
	}
}

func TestBumpNoCommitSkipsCommit(t *testing.T) {
	rec := &recordingBumper{}
	cmd := newCommand(rec)
	cmd.SetContext(app.WithContext(t.Context(), testApp(t)))
	cmd.SetArgs([]string{"test", "foo", "--no-commit"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if rec.commits != 0 {
		t.Errorf("commits = %d, want 0", rec.commits)
	}
}

func TestBumpUnknownRepoFails(t *testing.T) {
	cmd := newCommand(&recordingBumper{})
	cmd.SetContext(app.WithContext(t.Context(), testApp(t)))
	cmd.SetArgs([]string{"nope", "foo"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err == nil {
		t.Error("unknown repo should error")
	}
}
