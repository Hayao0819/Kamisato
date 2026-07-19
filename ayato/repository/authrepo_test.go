package repository

import "testing"

func newTestAuthRepo(t *testing.T) AuthRepository {
	t.Helper()
	return NewAuthRepository(newTestKV(t))
}

func TestAuthRepoFailClosed(t *testing.T) {
	r := newTestAuthRepo(t)
	if r.IsAdmin(1) {
		t.Fatalf("empty allowlist must deny id 1")
	}
	if r.IsAdmin(0) || r.IsAdmin(-5) {
		t.Fatalf("non-positive ids must be denied")
	}

	if err := r.AddAdmin(0, "x"); err == nil {
		t.Fatalf("AddAdmin(0) must be rejected")
	}
	if err := r.AddAdmin(-1, "x"); err == nil {
		t.Fatalf("AddAdmin(-1) must be rejected")
	}

	if err := r.AddAdmin(42, "alice"); err != nil {
		t.Fatalf("AddAdmin: %v", err)
	}
	if r.IsAdmin(99) {
		t.Fatalf("unknown id 99 must be denied")
	}
	if !r.IsAdmin(42) {
		t.Fatalf("added id 42 must be allowed")
	}
}

func TestAuthRepoRemove(t *testing.T) {
	r := newTestAuthRepo(t)
	_ = r.AddAdmin(5, "bob")
	if !r.IsAdmin(5) {
		t.Fatalf("id 5 must be allowed after AddAdmin")
	}
	if err := r.RemoveAdmin(5); err != nil {
		t.Fatalf("RemoveAdmin: %v", err)
	}
	if r.IsAdmin(5) {
		t.Fatalf("id 5 must be denied after RemoveAdmin")
	}
}

func TestAuthRepoList(t *testing.T) {
	r := newTestAuthRepo(t)
	_ = r.AddAdmin(1, "one")
	_ = r.AddAdmin(2, "two")
	admins, err := r.ListAdmins()
	if err != nil {
		t.Fatalf("ListAdmins: %v", err)
	}
	got := map[int64]string{}
	for _, a := range admins {
		got[a.ID] = a.Login
	}
	if got[1] != "one" || got[2] != "two" || len(got) != 2 {
		t.Fatalf("ListAdmins = %+v, want {1:one, 2:two}", got)
	}
}
