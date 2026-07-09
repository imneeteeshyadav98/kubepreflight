package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"kubepreflight/internal/findings"
	"kubepreflight/internal/plan"
)

func TestRenderActionPlanMarkdownIncludesChangeTicketChecklist(t *testing.T) {
	r := findings.NewReport("1.30", "prod", "eks", time.Now(), []findings.Finding{
		{
			RuleID:      "API-001",
			Severity:    findings.SeverityBlocker,
			Confidence:  findings.TierStaticCertain,
			Message:     "manifest uses a removed API",
			Resources:   []findings.ResourceReference{findings.ManifestResource("PodDisruptionBudget", findings.ScopeNamespaced, "default", "old-pdb", "manifests/pdb.yaml")},
			Fingerprint: "fp-api-001",
		},
	})
	actionPlan := plan.BuildActionPlan(r, time.Date(2026, 7, 9, 1, 2, 3, 0, time.UTC))

	var buf bytes.Buffer
	if err := WriteActionPlanMarkdown(actionPlan, &buf); err != nil {
		t.Fatalf("WriteActionPlanMarkdown: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"# Upgrade Execution Plan",
		"## Change Ticket Checklist",
		"### Phase 1 - Critical Blockers",
		"- [ ] **Fix removed or deprecated API usage** (`required`, required)",
		"Source rules: `API-001`",
		"### Phase 2 - Upgrade Preparation",
		"### Phase 3 - Upgrade",
		"- [ ] **Upgrade control plane** (`blocked`, required)",
		"Blocked until critical upgrade blockers are resolved.",
		"### Phase 4 - Validation",
		"- [ ] **Run smoke tests** (`manual`, required)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown missing %q\n--- full output ---\n%s", want, out)
		}
	}
}

func TestWriteActionPlanJSON(t *testing.T) {
	actionPlan := plan.BuildActionPlan(nil, time.Date(2026, 7, 9, 1, 2, 3, 0, time.UTC))

	var buf bytes.Buffer
	if err := WriteActionPlanJSON(actionPlan, &buf); err != nil {
		t.Fatalf("WriteActionPlanJSON: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		`"schemaVersion": "kubepreflight.io/upgrade-action-plan/v1"`,
		`"phases": [`,
		`"title": "Phase 2 - Upgrade Preparation"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("json missing %q\n--- full output ---\n%s", want, out)
		}
	}
}
