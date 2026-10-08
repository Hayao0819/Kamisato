package gccmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/service"
)

type fakeReconciler struct {
	dryRun bool
	repo   string
	age    time.Duration
}

func (f *fakeReconciler) ReconcileOrphans(repo string, age time.Duration, dryRun bool) ([]service.OrphanObject, error) {
	f.repo, f.age, f.dryRun = repo, age, dryRun
	return []service.OrphanObject{{Arch: "x86_64", Name: "old.pkg.tar.zst", Age: 2 * time.Hour}}, nil
}

func TestRunPassesDeletionPolicyToService(t *testing.T) {
	for _, deleteObjects := range []bool{false, true} {
		var out bytes.Buffer
		svc := &fakeReconciler{}
		if err := run(&out, svc, "extra", time.Hour, deleteObjects); err != nil {
			t.Fatal(err)
		}
		if svc.repo != "extra" || svc.age != time.Hour || svc.dryRun != !deleteObjects {
			t.Fatalf("service inputs = %#v", svc)
		}
		if !strings.Contains(out.String(), "orphan objects: 1") || strings.Contains(out.String(), "re-run with --delete") != !deleteObjects {
			t.Fatalf("output = %q", out.String())
		}
	}
}
