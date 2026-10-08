package domain

import (
	"testing"
	"time"
)

func TestJobCloneOwnsMutableRequestAndResultData(t *testing.T) {
	check, verify := true, true
	started, ended := time.Now(), time.Now()
	job := &BuildJob{
		Packages: []string{"original"}, StartedAt: &started, EndedAt: &ended,
		Request: &BuildRequest{Git: &GitSource{URL: "original"}, Files: map[string]string{"source": "original"}, InstallPkgs: []string{"original"}, RunCheck: &check, RunVerify: &verify},
	}
	copy := job.Clone()
	copy.Packages[0] = "changed"
	copy.Request.Git.URL = "changed"
	copy.Request.Files["source"] = "changed"
	copy.Request.InstallPkgs[0] = "changed"
	*copy.Request.RunCheck, *copy.Request.RunVerify = false, false
	*copy.StartedAt, *copy.EndedAt = time.Time{}, time.Time{}
	if job.Packages[0] != "original" || job.Request.Git.URL != "original" || job.Request.Files["source"] != "original" || job.Request.InstallPkgs[0] != "original" {
		t.Fatal("snapshot shares mutable source data")
	}
	if !*job.Request.RunCheck || !*job.Request.RunVerify || job.StartedAt.IsZero() || job.EndedAt.IsZero() {
		t.Fatal("snapshot shares pointer values")
	}
	if (*BuildJob)(nil).Clone() != nil || (*BuildRequest)(nil).Clone() != nil {
		t.Fatal("nil clones must stay nil")
	}
}
