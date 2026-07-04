package report

import (
	"bytes"
	"encoding/json"
	"testing"

	"kubepreflight/internal/plan"
)

func TestWritePlanJSON_RoundTrips(t *testing.T) {
	p := samplePlanReport(t)
	var buf bytes.Buffer
	if err := WritePlanJSON(p, &buf); err != nil {
		t.Fatalf("WritePlanJSON: %v", err)
	}

	var decoded plan.PlanReport
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decoding written JSON: %v", err)
	}
	if decoded.FromVersion != p.FromVersion || decoded.ToVersion != p.ToVersion || decoded.Provider != p.Provider {
		t.Errorf("decoded PlanReport = %+v, want fields matching %+v", decoded, p)
	}
	if len(decoded.Hops) != len(p.Hops) {
		t.Fatalf("decoded %d hops, want %d", len(decoded.Hops), len(p.Hops))
	}
	if decoded.Hops[0].Status != plan.HopStatusExact {
		t.Errorf("decoded Hops[0].Status = %v, want HopStatusExact", decoded.Hops[0].Status)
	}
	if decoded.Hops[0].Report == nil || len(decoded.Hops[0].Report.Findings) != len(p.Hops[0].Report.Findings) {
		t.Errorf("decoded hop 1 findings did not round-trip")
	}
}
