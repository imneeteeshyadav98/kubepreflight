package report

import (
	"encoding/json"
	"io"

	"kubepreflight/internal/plan"
)

// WritePlanJSON serializes a PlanReport as indented JSON to w — the
// canonical upgrade-plan.json format. Kept in internal/report (not
// internal/plan) for the same reason WriteJSON/WriteHTML live here rather
// than in internal/findings: internal/plan stays free of I/O concerns.
func WritePlanJSON(p *plan.PlanReport, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(p)
}
