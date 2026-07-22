package listcmd

import (
	"io"

	"github.com/Hayao0819/Kamisato/ayaka/service/report"
	"github.com/Hayao0819/Kamisato/internal/cliutil"
)

// pkgHeader is run through the format template to produce the table header row, as Docker does.
var pkgHeader = report.Row{
	Repo:      "REPO",
	Package:   "PACKAGE",
	Installed: "INSTALLED",
	Local:     "LOCAL",
	Remote:    "REMOTE",
	Build:     "BUILD",
}

func renderRows(out io.Writer, format string, rows []report.Row) error {
	return cliutil.RenderList(out, format, pkgHeader, rows)
}
