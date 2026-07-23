package matrixcmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/ayaka/service/plan"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

type recordingPlanner struct {
	plans   map[string]*plan.Plan
	arches  []string
	reloads int
}

func (r *recordingPlanner) Compute(_ []*pkg.SourcePackage, _ *repo.RemoteRepo, arch string, _ plan.CascadeMode, _ int, _ map[string]float64) (*plan.Plan, error) {
	r.arches = append(r.arches, arch)
	if p, ok := r.plans[arch]; ok {
		return p, nil
	}
	return &plan.Plan{Order: []string{}, Reasons: map[string]string{}, BumpTargets: []string{}}, nil
}

func (r *recordingPlanner) ReloadWithSrcinfo(srcrepo *repo.SourceRepo, _ io.Writer) (*repo.SourceRepo, error) {
	r.reloads++
	return srcrepo, nil
}

// testApp wires the source repo's url to a local server that always answers
// 404, so shared.RemoteRepo takes its "treat as empty" path instead of
// reaching the network.
func testApp(t *testing.T, arches []string) *app.App {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return &app.App{SrcRepos: []*repo.SourceRepo{{
		Config: &repo.SrcConfig{Name: "test", URL: srv.URL, Build: builder.ProjectConfig{Arches: arches}},
	}}}
}

func run(t *testing.T, rec *recordingPlanner, a *app.App, args ...string) *Matrix {
	t.Helper()
	cmd := newCommand(rec)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetContext(app.WithContext(t.Context(), a))
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var m Matrix
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	return &m
}

func TestMatrixBucketsPerArch(t *testing.T) {
	rec := &recordingPlanner{plans: map[string]*plan.Plan{
		"i486": {Order: []string{"a", "b"}, Buckets: [][]string{{"a", "b"}}, BumpTargets: []string{"b"}},
		"i686": {Order: []string{"a"}, Buckets: [][]string{{"a"}}, BumpTargets: []string{"a"}},
	}}
	m := run(t, rec, testApp(t, []string{"i486", "i686"}), "--update-srcinfo=false")

	if len(m.BuildMatrix.Include) != 2 {
		t.Fatalf("build entries = %d, want 2: %+v", len(m.BuildMatrix.Include), m.BuildMatrix.Include)
	}
	if e := m.BuildMatrix.Include[0]; e.Repo != "test" || e.Arch != "i486" || e.Pkgs != "a b" {
		t.Errorf("unexpected first build entry: %+v", e)
	}
	if len(m.PruneMatrix.Include) != 2 {
		t.Errorf("prune entries = %d, want 2", len(m.PruneMatrix.Include))
	}
	// pkgrel is arch-independent, so bump targets union across arches.
	if got := m.Bumps["test"]; got != "b a" && got != "a b" {
		t.Errorf("bumps = %q, want union of a and b", got)
	}
	if !m.AnyBuild {
		t.Error("any_build should be true")
	}
}

func TestMatrixEmptyPlan(t *testing.T) {
	rec := &recordingPlanner{}
	m := run(t, rec, testApp(t, nil), "--update-srcinfo=false")
	if m.AnyBuild {
		t.Error("any_build should be false with nothing to build")
	}
	if len(m.Bumps) != 0 {
		t.Errorf("bumps should be empty, got %v", m.Bumps)
	}
	// No arches in repo.json defaults to x86_64.
	if len(rec.arches) != 1 || rec.arches[0] != "x86_64" {
		t.Errorf("planned arches = %v, want [x86_64]", rec.arches)
	}
}

func TestMatrixForceModeSkipsPlan(t *testing.T) {
	rec := &recordingPlanner{}
	m := run(t, rec, testApp(t, []string{"i486", "i686"}), "--packages", "foo bar")
	if len(rec.arches) != 0 || rec.reloads != 0 {
		t.Errorf("force mode must not plan or reload (planned %v, reloads %d)", rec.arches, rec.reloads)
	}
	if len(m.BuildMatrix.Include) != 2 {
		t.Fatalf("build entries = %d, want one per arch", len(m.BuildMatrix.Include))
	}
	if e := m.BuildMatrix.Include[0]; e.Pkgs != "foo bar" {
		t.Errorf("forced pkgs = %q, want %q", e.Pkgs, "foo bar")
	}
}

func TestMatrixWorkersZeroFallsBackToOneBucket(t *testing.T) {
	rec := &recordingPlanner{plans: map[string]*plan.Plan{
		"x86_64": {Order: []string{"a", "b"}, BumpTargets: []string{}},
	}}
	m := run(t, rec, testApp(t, nil), "--update-srcinfo=false")
	if len(m.BuildMatrix.Include) != 1 || m.BuildMatrix.Include[0].Pkgs != "a b" {
		t.Errorf("want the flat order as one bucket, got %+v", m.BuildMatrix.Include)
	}
}

func TestMatrixGithubOutput(t *testing.T) {
	outFile := filepath.Join(t.TempDir(), "out")
	t.Setenv("GITHUB_OUTPUT", outFile)
	rec := &recordingPlanner{plans: map[string]*plan.Plan{
		"x86_64": {Order: []string{"a"}, Buckets: [][]string{{"a"}}, BumpTargets: []string{"a"}},
	}}
	cmd := newCommand(rec)
	cmd.SetContext(app.WithContext(t.Context(), testApp(t, nil)))
	cmd.SetArgs([]string{"--update-srcinfo=false", "--format", "github"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		`build_matrix={"include":[{"repo":"test","arch":"x86_64","pkgs":"a"}]}`,
		`prune_matrix={"include":[{"repo":"test","arch":"x86_64"}]}`,
		`bumps={"test":"a"}`,
		"any_build=true",
	} {
		if !strings.Contains(got, want+"\n") {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}
