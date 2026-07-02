package listcmd

import (
	"io"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
)

// pkgHeader is run through the format template to produce the table header row, as Docker does.
var pkgHeader = shared.PkgRow{
	Repo:      "REPO",
	Package:   "PACKAGE",
	Installed: "INSTALLED",
	Local:     "LOCAL",
	Remote:    "REMOTE",
	Build:     "BUILD",
}

// renderRows renders package rows through the shared Docker-style formatter.
func renderRows(out io.Writer, format string, rows []shared.PkgRow) error {
	return shared.RenderList(out, format, pkgHeader, rows)
}
