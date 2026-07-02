package conf

import "testing"

func TestAyakaValidate(t *testing.T) {
	if err := (&AyakaConfig{}).Validate(); err != nil {
		t.Errorf("empty config (legacy/fresh) rejected: %v", err)
	}
	if err := (&AyakaConfig{Repos: []RepoEntry{{Dir: "myrepo"}}}).Validate(); err != nil {
		t.Errorf("valid repo entry rejected: %v", err)
	}
	if err := (&AyakaConfig{Repos: []RepoEntry{{DestDir: "out"}}}).Validate(); err == nil {
		t.Error("expected an error for a repo entry with no dir")
	}
}

func TestSrcRepoValidate(t *testing.T) {
	if err := (&SrcRepoConfig{Name: "myrepo"}).Validate(); err != nil {
		t.Errorf("valid src repo rejected: %v", err)
	}
	if err := (&SrcRepoConfig{}).Validate(); err == nil {
		t.Error("expected an error for a src repo with no name")
	}
}
