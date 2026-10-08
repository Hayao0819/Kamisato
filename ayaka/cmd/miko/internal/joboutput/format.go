package joboutput

import miko "github.com/Hayao0819/Kamisato/miko/client"

const TableFormat = "table {{.ID}}\t{{.Repo}}\t{{.Arch}}\t{{.Status}}\t{{.CreatedAt}}"

var Header = miko.Job{ID: "ID", Repo: "REPO", Arch: "ARCH", Status: "STATUS", CreatedAt: "CREATED"}
