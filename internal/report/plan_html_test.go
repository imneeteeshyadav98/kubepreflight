package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"kubepreflight/internal/findings"
	"kubepreflight/internal/plan"
)

func TestWritePlanHTML_RendersVerdictAndUpgradePath(t *testing.T) {
	p := samplePlanReport(t)
	var buf bytes.Buffer
	if err := WritePlanHTML(p, &buf); err != nil {
		t.Fatalf("WritePlanHTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		`class="plan-verdict-banner blocked"`,
		"NOT READY FOR UPGRADE",
		"1 blocker(s) found",
		"Upgrade Path (1.29",
		`class="badge-current-live"`,
		"Current live",
		`class="badge-projected"`,
		"Projected",
		`class="badge-rescan-required"`,
		"Rescan required",
		"NODE-001: nodes may be replaced",
		"Future-hop findings are projections",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("plan HTML output missing %q", want)
		}
	}

	// The Blockers/Warnings/Next Actions sections must still render hop
	// 1's findings identically to a plain scan report.
	for _, want := range []string{"PDB-001", "WH-001", "Scale up replicas."} {
		if !strings.Contains(out, want) {
			t.Errorf("plan HTML output missing hop-1 finding detail %q", want)
		}
	}
}

func TestWritePlanHTML_GlobalBlockerVerdict(t *testing.T) {
	hop1 := findings.NewReport("1.30", "prod-cluster", "eks", time.Now(), []findings.Finding{
		{
			RuleID: "WH-002", Severity: findings.SeverityBlocker, Confidence: findings.TierStaticCertain,
			Message:       "webhook is fail-closed with zero endpoints",
			Resources:     []findings.ResourceReference{findings.LiveResource("ValidatingWebhookConfiguration", findings.ScopeCluster, "", "guard", "uid-1")},
			GlobalBlocker: true,
			Fingerprint:   "fp-wh002",
		},
	})
	hops := []plan.Hop{{Index: 1, From: "1.29", To: "1.30"}}
	p, err := plan.BuildPlan("prod-cluster", "eks", "1.29", "explicit-flag", "1.30", hops, hop1, nil, time.Now())
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}

	var buf bytes.Buffer
	if err := WritePlanHTML(p, &buf); err != nil {
		t.Fatalf("WritePlanHTML: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Global API write blocker detected") {
		t.Errorf("plan HTML output missing global-blocker verdict reason")
	}
}

func TestWritePlanHTML_CleanPlanIsReady(t *testing.T) {
	hop1 := findings.NewReport("1.30", "prod-cluster", "eks", time.Now(), nil)
	hops := []plan.Hop{{Index: 1, From: "1.29", To: "1.30"}}
	p, err := plan.BuildPlan("prod-cluster", "eks", "1.29", "explicit-flag", "1.30", hops, hop1, nil, time.Now())
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}

	var buf bytes.Buffer
	if err := WritePlanHTML(p, &buf); err != nil {
		t.Fatalf("WritePlanHTML: %v", err)
	}
	out := buf.String()
	for _, want := range []string{`class="plan-verdict-banner clean"`, "No known upgrade blockers detected"} {
		if !strings.Contains(out, want) {
			t.Errorf("plan HTML output missing %q", want)
		}
	}
}

// TestWriteHTML_ScanReportsNeverShowPlanMarkup proves the {{if .Plan}}
// gate actually hides the Upgrade Path section for ordinary scan output —
// WriteHTML never sets htmlViewData.Plan.
func TestWriteHTML_ScanReportsNeverShowPlanMarkup(t *testing.T) {
	rpt := sampleReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()
	// CSS rules for these classes are always present statically (part of
	// the shared <style> block) — what must be absent is the *rendered*
	// {{if .Plan}} section content itself.
	for _, unwanted := range []string{"Upgrade Path (", "NOT READY FOR UPGRADE", "CONDITIONALLY READY", "Future-hop findings are projections"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("scan HTML output unexpectedly contains %q", unwanted)
		}
	}
}
