package plancmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/ayaka/service/plan"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

type recordingPlanner struct {
	arch    string
	cascade plan.CascadeMode
	workers int
	reload  int
}

func (r *recordingPlanner) Compute(_ []*pkg.SourcePackage, _ *repo.RemoteRepo, arch string, cascade plan.CascadeMode, workers int, _ map[string]float64) (*plan.Plan, error) {
	r.arch = arch
	r.cascade = cascade
	r.workers = workers
	return &plan.Plan{Order: []string{"foo"}, Reasons: map[string]string{"foo": "version"}, BumpTargets: []string{}}, nil
}

func (r *recordingPlanner) ReloadWithSrcinfo(srcrepo *repo.SourceRepo, _ io.Writer) (*repo.SourceRepo, error) {
	r.reload++
	return srcrepo, nil
}

// testApp wires the source repo's url to a local server that always answers
// 404, so shared.RemoteRepo takes its "treat as empty" path instead of
// reaching the network.
func testApp(t *testing.T) *app.App {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return &app.App{SrcRepos: []*repo.SourceRepo{{
		Config: &repo.SrcConfig{Name: "test", URL: srv.URL},
	}}}
}

func TestPlanFlagsReachService(t *testing.T) {
	rec := &recordingPlanner{}
	cmd := newCommand(rec)
	cmd.SetContext(app.WithContext(t.Context(), testApp(t)))
	cmd.SetArgs([]string{"test", "--arch", "aarch64", "--cascade", "soname", "--workers", "3", "--update-srcinfo=false"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if rec.arch != "aarch64" || rec.cascade != plan.CascadeSoname || rec.workers != 3 {
		t.Errorf("service got arch=%q cascade=%q workers=%d", rec.arch, rec.cascade, rec.workers)
	}
	if rec.reload != 0 {
		t.Errorf("reload should be skipped with --update-srcinfo=false, got %d calls", rec.reload)
	}
}

func TestPlanUpdateSrcinfoDefaultOn(t *testing.T) {
	rec := &recordingPlanner{}
	cmd := newCommand(rec)
	cmd.SetContext(app.WithContext(t.Context(), testApp(t)))
	cmd.SetArgs([]string{"test"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if rec.reload != 1 {
		t.Errorf("reload calls = %d, want 1", rec.reload)
	}
}

func TestPlanUnknownRepoFails(t *testing.T) {
	cmd := newCommand(&recordingPlanner{})
	cmd.SetContext(app.WithContext(t.Context(), testApp(t)))
	cmd.SetArgs([]string{"nope"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err == nil {
		t.Error("unknown repo should error")
	}
}

func TestPlanNoURLFails(t *testing.T) {
	cmd := newCommand(&recordingPlanner{})
	a := &app.App{SrcRepos: []*repo.SourceRepo{{Config: &repo.SrcConfig{Name: "test"}}}}
	cmd.SetContext(app.WithContext(t.Context(), a))
	cmd.SetArgs([]string{"test"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err == nil {
		t.Error("missing repo.json url should error")
	}
}
