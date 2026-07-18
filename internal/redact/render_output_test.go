package redact

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"kubepreflight/internal/comparison"
	"kubepreflight/internal/findings"
	"kubepreflight/internal/plan"
	"kubepreflight/internal/report"
)

func assertRenderedOutputRedacted(t *testing.T, label string, raw string) {
	t.Helper()
	if strings.Contains(raw, realARN) {
		t.Fatalf("%s leaked real ARN %q in rendered output:\n%s", label, realARN, raw)
	}
	if strings.Contains(raw, realHostname) {
		t.Fatalf("%s leaked real node hostname %q in rendered output:\n%s", label, realHostname, raw)
	}
	if !strings.Contains(raw, ARNPlaceholder) && !strings.Contains(raw, HostnamePlaceholder) {
		t.Fatalf("%s did not contain either redaction placeholder; fixture may no longer exercise redaction:\n%s", label, raw)
	}
}

func renderToString(t *testing.T, label string, render func(*bytes.Buffer) error) string {
	t.Helper()
	var buf bytes.Buffer
	if err := render(&buf); err != nil {
		t.Fatalf("%s render failed: %v", label, err)
	}
	return buf.String()
}

func realPlanReport() *plan.PlanReport {
	return &plan.PlanReport{
		SchemaVersion:     "kubepreflight.io/upgrade-plan/v1alpha1",
		ClusterContext:    realARN,
		Provider:          "eks",
		FromVersion:       "1.35",
		FromVersionSource: "explicit-flag",
		ToVersion:         "1.36",
		Hops: []plan.HopReport{
			{
				Hop:    plan.Hop{From: "1.35", To: "1.36"},
				Status: plan.HopStatusExact,
				Report: realReport(),
			},
		},
		ActionPlan: &plan.UpgradeActionPlan{
			SchemaVersion: findings.SchemaVersion,
			Verdict:       "BLOCKED",
			Phases: []plan.ActionPhase{
				{
					ID:          "phase-1",
					Title:       "Fix blockers",
					Description: "resource " + realARN,
					Gate:        "node " + realHostname + " is remediated",
					Actions: []plan.PlanAction{
						{
							ID:              "fix-node",
							Title:           "Fix " + realHostname,
							Reason:          "Evidence references " + realARN,
							SuccessCriteria: []string{"no findings mention " + realHostname},
							Commands:        []string{"kubectl get node " + realHostname},
						},
					},
				},
			},
		},
	}
}

func TestReportRedaction_ReachesRenderedJSONMarkdownHTMLAndTerminal(t *testing.T) {
	r := realReport()
	Report(r)

	outputs := map[string]string{
		"findings.json": renderToString(t, "findings.json", func(buf *bytes.Buffer) error {
			return report.WriteJSON(r, buf)
		}),
		"report.md": renderToString(t, "report.md", func(buf *bytes.Buffer) error {
			return report.WriteMarkdown(r, buf)
		}),
		"report.html": renderToString(t, "report.html", func(buf *bytes.Buffer) error {
			return report.WriteHTML(r, buf)
		}),
		"terminal": renderToString(t, "terminal", func(buf *bytes.Buffer) error {
			return report.WriteTerminal(r, buf)
		}),
	}

	for label, raw := range outputs {
		assertRenderedOutputRedacted(t, label, raw)
	}
}

func TestPlanRedaction_ReachesRenderedJSONHTMLAndTerminal(t *testing.T) {
	pr := realPlanReport()
	PlanReport(pr)

	outputs := map[string]string{
		"upgrade-plan.json": renderToString(t, "upgrade-plan.json", func(buf *bytes.Buffer) error {
			return report.WritePlanJSON(pr, buf)
		}),
		"plan-report.html": renderToString(t, "plan-report.html", func(buf *bytes.Buffer) error {
			return report.WritePlanHTML(pr, buf)
		}),
		"plan-terminal": renderToString(t, "plan-terminal", func(buf *bytes.Buffer) error {
			if err := report.WriteTerminal(pr.Hops[0].Report, buf); err != nil {
				return err
			}
			return report.WritePlanCompactSummary(pr, buf)
		}),
	}

	for label, raw := range outputs {
		assertRenderedOutputRedacted(t, label, raw)
	}
}

func TestRollbackRedaction_ReachesRenderedJSONMarkdownHTMLAndTerminal(t *testing.T) {
	a := realAssessment()
	RollbackAssessment(&a)

	outputs := map[string]string{
		"rollback-assessment.json": renderToString(t, "rollback-assessment.json", func(buf *bytes.Buffer) error {
			return report.WriteRollbackJSON(&a, buf)
		}),
		"rollback-report.md": renderToString(t, "rollback-report.md", func(buf *bytes.Buffer) error {
			return report.WriteRollbackMarkdown(&a, buf)
		}),
		"rollback-report.html": renderToString(t, "rollback-report.html", func(buf *bytes.Buffer) error {
			return report.WriteRollbackHTML(&a, buf)
		}),
		"rollback-terminal": renderToString(t, "rollback-terminal", func(buf *bytes.Buffer) error {
			return report.WriteRollbackTerminal(&a, buf)
		}),
	}

	for label, raw := range outputs {
		assertRenderedOutputRedacted(t, label, raw)
	}
}

func TestComparisonRedaction_ReachesRenderedJSONMarkdownAndTerminalSummary(t *testing.T) {
	c := realComparison()
	Comparison(c)

	jsonOutput := renderToString(t, "comparison.json", func(buf *bytes.Buffer) error {
		enc := json.NewEncoder(buf)
		enc.SetIndent("", "  ")
		return enc.Encode(c)
	})
	markdownOutput := renderToString(t, "comparison.md", func(buf *bytes.Buffer) error {
		return comparison.WriteMarkdown(c, buf)
	})
	terminalOutput := fmt.Sprintf("Comparison: %s -> %s\nReadiness score: %d -> %d\nWarning: %s\n",
		c.Summary.BaselineVerdict,
		c.Summary.CurrentVerdict,
		c.Summary.BaselineReadinessScore,
		c.Summary.CurrentReadinessScore,
		c.Warnings[0],
	)

	for label, raw := range map[string]string{
		"comparison.json":     jsonOutput,
		"comparison.md":       markdownOutput,
		"comparison-terminal": terminalOutput,
	} {
		assertRenderedOutputRedacted(t, label, raw)
	}
}
