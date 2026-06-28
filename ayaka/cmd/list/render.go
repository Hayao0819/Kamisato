package listcmd

import (
	"encoding/json"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/spf13/cobra"
)

// pkgHeader is rendered through the format template to produce the table header
// row, the same way Docker derives headers from the format string.
var pkgHeader = shared.PkgRow{
	Repo:      "REPO",
	Package:   "PACKAGE",
	Installed: "INSTALLED",
	Local:     "LOCAL",
	Remote:    "REMOTE",
	Build:     "BUILD",
}

// renderRows writes the rows according to the format string. "json" emits one
// JSON object per line; a "table " prefix aligns the columns under a header;
// any other template is executed per row with no header.
func renderRows(cmd *cobra.Command, format string, rows []shared.PkgRow) error {
	out := cmd.OutOrStdout()

	if format == "json" {
		enc := json.NewEncoder(out)
		for _, row := range rows {
			if err := enc.Encode(row); err != nil {
				return utils.WrapErr(err, "failed to encode row")
			}
		}
		return nil
	}

	// Let users write \t and \n in the format string, as docker does.
	tmplText := strings.NewReplacer(`\t`, "\t", `\n`, "\n").Replace(format)

	isTable := strings.HasPrefix(tmplText, "table ")
	if isTable {
		tmplText = strings.TrimPrefix(tmplText, "table ")
	}

	tmpl, err := template.New("list").Funcs(template.FuncMap{
		"json": func(v any) (string, error) {
			b, err := json.Marshal(v)
			return string(b), err
		},
	}).Parse(tmplText)
	if err != nil {
		return utils.WrapErr(err, "invalid --format template")
	}

	if isTable {
		w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
		if err := tmpl.Execute(w, pkgHeader); err != nil {
			return utils.WrapErr(err, "failed to render header")
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return err
		}
		for _, row := range rows {
			if err := tmpl.Execute(w, row); err != nil {
				return utils.WrapErr(err, "failed to render row")
			}
			if _, err := w.Write([]byte("\n")); err != nil {
				return err
			}
		}
		return w.Flush()
	}

	for _, row := range rows {
		if err := tmpl.Execute(out, row); err != nil {
			return utils.WrapErr(err, "failed to render row")
		}
		if _, err := out.Write([]byte("\n")); err != nil {
			return err
		}
	}
	return nil
}
