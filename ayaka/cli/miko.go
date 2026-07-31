package cli

import "github.com/Hayao0819/Kamisato/internal/mikoapi"

const JobTableFormat = "table {{.ID}}\t{{.Repo}}\t{{.Arch}}\t{{.Status}}\t{{.CreatedAt}}"

var JobHeader = mikoapi.Job{ID: "ID", Repo: "REPO", Arch: "ARCH", Status: "STATUS", CreatedAt: "CREATED"}
