package cliutil

import (
	"encoding/json"
	"io"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/spf13/cobra"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

// AddFormatFlags registers the shared --format/-F and --json output flags used
// by the list-like commands, so every one of them speaks the same Docker-style
// templating and offers the same JSON escape hatch for scripting.
func AddFormatFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("format", "F", "", "Format output with a Go template (Docker-style; 'table ...' or 'json')")
	cmd.Flags().Bool("json", false, "Output JSON (shorthand for --format json)")
}

// ResolveFormat reads --format/--json and returns the effective format, applying
// def when neither is set. --json wins and is equivalent to --format json.
func ResolveFormat(cmd *cobra.Command, def string) (string, error) {
	asJSON, err := cmd.Flags().GetBool("json")
	if err != nil {
		return "", err
	}
	if asJSON {
		return "json", nil
	}
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return "", err
	}
	if format == "" {
		return def, nil
	}
	return format, nil
}

// RenderList writes rows per format: "json" emits one object per line, a "table "
// prefix aligns columns under header, and any other template runs per row with no
// header. header is a row whose fields carry the column labels for the table form.
func RenderList[T any](out io.Writer, format string, header T, rows []T) error {
	if format == "json" {
		enc := json.NewEncoder(out)
		for _, row := range rows {
			if err := enc.Encode(row); err != nil {
				return errors.WrapErr(err, "failed to encode row")
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
		return errors.WrapErr(err, "invalid --format template")
	}

	if isTable {
		w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
		if err := tmpl.Execute(w, header); err != nil {
			return errors.WrapErr(err, "failed to render header")
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return err
		}
		if err := renderRows(w, tmpl, rows); err != nil {
			return err
		}
		return w.Flush()
	}

	return renderRows(out, tmpl, rows)
}

func renderRows[T any](out io.Writer, tmpl *template.Template, rows []T) error {
	for _, row := range rows {
		if err := tmpl.Execute(out, row); err != nil {
			return errors.WrapErr(err, "failed to render row")
		}
		if _, err := out.Write([]byte("\n")); err != nil {
			return err
		}
	}
	return nil
}
