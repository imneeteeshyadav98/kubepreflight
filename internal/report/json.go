// Package report renders a findings.Report to an output format. Only the
// canonical JSON writer ships in Week 1; terminal/Markdown/HTML renderers
// are Week 4 scope.
package report

import (
	"encoding/json"
	"io"

	"kubepreflight/internal/findings"
)

// WriteJSON serializes a Report as indented JSON to w. This is the
// canonical findings.json format other tooling (CI, SARIF conversion,
// SaaS ingestion) is expected to consume.
func WriteJSON(r *findings.Report, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
