package handler

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/internal/protocol"
	"github.com/Hayao0819/Kamisato/miko/domain"
)

func TestProtocolBuildRequestDoesNotAcceptRequester(t *testing.T) {
	var request protocol.BuildRequest
	if err := json.Unmarshal([]byte(`{"repo":"core","arch":"x86_64","requester":"attacker"}`), &request); err != nil {
		t.Fatal(err)
	}
	domainRequest := domainBuildRequest(&request)
	if domainRequest.Requester != "" {
		t.Fatalf("wire requester reached domain model: %q", domainRequest.Requester)
	}
}

func TestProtocolBuildJobExcludesPersistenceFields(t *testing.T) {
	started := time.Date(2026, 7, 16, 10, 20, 30, 123, time.UTC)
	job := &domain.BuildJob{
		ID:          "job-1",
		Repo:        "core",
		Arch:        "x86_64",
		Status:      domain.JobStatusRunning,
		Reason:      domain.ReasonDependency,
		Owner:       "service-a",
		Request:     &domain.BuildRequest{Pkgbuild: "secret execution input"},
		ArtifactDir: "/private/artifacts/job-1",
		CreatedAt:   started.Add(-time.Minute),
		StartedAt:   &started,
	}

	wire := protocolBuildJob(job)
	raw, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(raw)
	for _, forbidden := range []string{"service-a", "secret execution input", "/private/artifacts", "owner", "request", "artifact_dir"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("wire job leaks %q: %s", forbidden, encoded)
		}
	}
	if wire.Reason != protocol.ReasonDependency || wire.StartedAt == nil || *wire.StartedAt != started.Format(time.RFC3339Nano) {
		t.Fatalf("wire projection lost public fields: %#v", wire)
	}
}
