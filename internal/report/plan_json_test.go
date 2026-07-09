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
	if decoded.SchemaVersion == "" {
		t.Error("plan schemaVersion is empty")
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
	if decoded.ActionPlan == nil {
		t.Fatal("decoded ActionPlan is nil")
	}
	if decoded.ActionPlan.SchemaVersion != plan.ActionPlanSchemaVersion {
		t.Errorf("decoded actionPlan.schemaVersion = %q, want %q", decoded.ActionPlan.SchemaVersion, plan.ActionPlanSchemaVersion)
	}
	if len(decoded.ActionPlan.Phases) != 4 {
		t.Errorf("decoded actionPlan phases = %d, want 4", len(decoded.ActionPlan.Phases))
	}
}
