package mikocmd

import "github.com/Hayao0819/Kamisato/internal/ayatoclient"

// jobTableFormat is the default Docker-style table for job output, shared by
// `miko jobs` and `miko status` so the two agree column-for-column.
const jobTableFormat = "table {{.ID}}\t{{.Repo}}\t{{.Arch}}\t{{.Status}}\t{{.CreatedAt}}"

// jobHeader carries the column labels rendered as the job table's header row.
var jobHeader = ayatoclient.Job{ID: "ID", Repo: "REPO", Arch: "ARCH", Status: "STATUS", CreatedAt: "CREATED"}
