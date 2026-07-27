package nvbumpcmd

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

type recordingNvBumper struct {
	name     string
	version  string
	messages []string
}

func (r *recordingNvBumper) NvBump(src *repo.SourceRepo, name, newVersion string, _ io.Writer) (*pkg.SourcePackage, error) {
	r.name = name
	r.version = newVersion
	return src.Pkgs[0], nil
}

func (r *recordingNvBumper) Commit(_ string, _ []*pkg.SourcePackage, message string) (string, error) {
	r.messages = append(r.messages, message)
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

func TestNvBumpArgsReachService(t *testing.T) {
	rec := &recordingNvBumper{}
	cmd := newCommand(rec)
	cmd.SetContext(app.WithContext(t.Context(), testApp(t)))
	cmd.SetArgs([]string{"test", "foo", "2.0"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if rec.name != "foo" || rec.version != "2.0" {
		t.Errorf("service got name=%q version=%q", rec.name, rec.version)
	}
	if len(rec.messages) != 1 || rec.messages[0] != "chore: update foo to 2.0" {
		t.Errorf("commit messages = %v", rec.messages)
	}
}

func TestNvBumpCustomMessage(t *testing.T) {
	rec := &recordingNvBumper{}
	cmd := newCommand(rec)
	cmd.SetContext(app.WithContext(t.Context(), testApp(t)))
	cmd.SetArgs([]string{"test", "foo", "2.0", "--message", "custom"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(rec.messages) != 1 || rec.messages[0] != "custom" {
		t.Errorf("commit messages = %v, want [custom]", rec.messages)
	}
}

func TestNvBumpNoCommitSkipsCommit(t *testing.T) {
	rec := &recordingNvBumper{}
	cmd := newCommand(rec)
	cmd.SetContext(app.WithContext(t.Context(), testApp(t)))
	cmd.SetArgs([]string{"test", "foo", "2.0", "--no-commit"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(rec.messages) != 0 {
		t.Errorf("commit messages = %v, want none", rec.messages)
	}
}

func TestNvBumpUnknownRepoFails(t *testing.T) {
	cmd := newCommand(&recordingNvBumper{})
	cmd.SetContext(app.WithContext(t.Context(), testApp(t)))
	cmd.SetArgs([]string{"nope", "foo", "2.0"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err == nil {
		t.Error("unknown repo should error")
	}
}
