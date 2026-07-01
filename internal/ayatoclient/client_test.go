package ayatoclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSubmitBuildSendsBearer(t *testing.T) {
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{"job_id": "job-1"})
	}))
	defer srv.Close()

	id, err := SubmitBuild(context.Background(), srv.URL, "tok123", &BuildRequest{Repo: "r", Arch: "x86_64"})
	if err != nil {
		t.Fatalf("SubmitBuild: %v", err)
	}
	if id != "job-1" {
		t.Fatalf("job id = %q, want job-1", id)
	}
	if gotAuth != "Bearer tok123" {
		t.Fatalf("auth = %q, want Bearer tok123", gotAuth)
	}
	if gotPath != "/api/unstable/build" {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestCancelJobSendsBearer(t *testing.T) {
	var gotAuth, gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := CancelJob(context.Background(), srv.URL, "tok123", "job-9"); err != nil {
		t.Fatalf("CancelJob: %v", err)
	}
	if gotAuth != "Bearer tok123" {
		t.Fatalf("auth = %q", gotAuth)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("method = %q", gotMethod)
	}
	if gotPath != "/api/unstable/jobs/job-9" {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestExchangeCLICodeRoundTrip(t *testing.T) {
	var body struct {
		Code         string `json:"code"`
		CodeVerifier string `json:"code_verifier"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "T", "login": "octocat", "id": 42})
	}))
	defer srv.Close()

	token, login, id, err := ExchangeCLICode(context.Background(), srv.URL, "the-code", "the-verifier")
	if err != nil {
		t.Fatalf("ExchangeCLICode: %v", err)
	}
	if token != "T" || login != "octocat" || id != 42 {
		t.Fatalf("got token=%q login=%q id=%d", token, login, id)
	}
	if body.Code != "the-code" || body.CodeVerifier != "the-verifier" {
		t.Fatalf("sent body = %+v", body)
	}
}

func TestAddAdminLoginVsID(t *testing.T) {
	cases := []struct {
		name      string
		id        int64
		login     string
		wantID    int64
		wantLogin string
	}{
		{"login", 0, "octocat", 0, "octocat"},
		{"id", 7, "", 7, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var sent struct {
				ID    int64  `json:"id"`
				Login string `json:"login"`
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(raw, &sent)
				_ = json.NewEncoder(w).Encode(map[string]any{"id": 99, "login": "resolved"})
			}))
			defer srv.Close()

			admin, err := AddAdmin(context.Background(), srv.URL, "tok", tc.id, tc.login)
			if err != nil {
				t.Fatalf("AddAdmin: %v", err)
			}
			if admin.ID != 99 || admin.Login != "resolved" {
				t.Fatalf("admin = %+v", admin)
			}
			if sent.ID != tc.wantID || sent.Login != tc.wantLogin {
				t.Fatalf("sent = %+v, want id=%d login=%q", sent, tc.wantID, tc.wantLogin)
			}
		})
	}
}

func TestRemoveAdminPath(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := RemoveAdmin(context.Background(), srv.URL, "tok", 13); err != nil {
		t.Fatalf("RemoveAdmin: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("method = %q", gotMethod)
	}
	if gotPath != "/api/unstable/auth/admins/13" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotAuth != "Bearer tok" {
		t.Fatalf("auth = %q", gotAuth)
	}
}
