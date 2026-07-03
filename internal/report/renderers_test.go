package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"kubepreflight/internal/findings"
)

func sampleReport() *findings.Report {
	fs := []findings.Finding{
		{
			RuleID: "WH-002", Severity: findings.SeverityBlocker, Confidence: findings.TierStaticCertain,
			Message:  `webhook "payments-guard" is fail-closed with no ready endpoints`,
			Resource: findings.Resource{Kind: "ValidatingWebhookConfiguration", Name: "payments-guard", UID: "uid-1"},
			Evidence: []string{"webhook index: 0", "ready endpoint address count: 0"},
			// Deliberately includes placeholder syntax like a real
			// remediation would (e.g. ADDON-001/API-001's `<cluster>`,
			// `<file>`) to exercise HTML escaping.
			Remediation: "Run: aws eks update-addon --cluster-name <cluster> --addon-name vpc-cni",
			Fingerprint: "fp-wh002",
		},
		{
			RuleID: "WH-001", Severity: findings.SeverityWarning, Confidence: findings.TierStaticCertain,
			Message:     `webhook "payments-guard" has catch-all scope`,
			Resource:    findings.Resource{Kind: "ValidatingWebhookConfiguration", Name: "payments-guard", UID: "uid-1"},
			Evidence:    []string{"scope: apiGroups=[\"*\"]"},
			Remediation: "Narrow the webhook's scope.",
			Fingerprint: "fp-wh001",
		},
	}
	return findings.NewReport("1.34", "prod-cluster", "eks", time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC), fs)
}

func TestWriteJSON_RoundTrips(t *testing.T) {
	rpt := sampleReport()
	var buf bytes.Buffer
	if err := WriteJSON(rpt, &buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var decoded findings.Report
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if decoded.TargetVersion != "1.34" || decoded.Provider != "eks" {
		t.Errorf("decoded report mismatch: %+v", decoded)
	}
	if len(decoded.Findings) != 2 {
		t.Errorf("got %d findings, want 2", len(decoded.Findings))
	}
}

func TestWriteTerminal_ContainsExpectedSections(t *testing.T) {
	rpt := sampleReport()
	var buf bytes.Buffer
	if err := WriteTerminal(rpt, &buf); err != nil {
		t.Fatalf("WriteTerminal: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"Result: BLOCKED",
		"Blockers (1)",
		"[WH-002]",
		"Warnings (1)",
		"[WH-001]",
		"Next Actions (1)", // WH-001+WH-002 on the same resource must merge to one action
		"Also see WH-001",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("terminal output missing %q\n--- full output ---\n%s", want, out)
		}
	}
}

func TestWriteMarkdown_ContainsExpectedSections(t *testing.T) {
	rpt := sampleReport()
	var buf bytes.Buffer
	if err := WriteMarkdown(rpt, &buf); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"# KubePreflight Scan Report",
		"## Blockers (1)",
		"## Warnings (1)",
		"## Next Actions (1)",
		"## Evidence Appendix",
		"```",
		"| Rule ID | Severity | Confidence | Resource | Fingerprint |",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown output missing %q", want)
		}
	}
}

// TestWriteHTML_EscapesPlaceholderSyntax guards the exact failure mode
// flagged during review: a naive text/template or string-concat renderer
// would let a browser interpret "<cluster>" in remediation text as an
// unknown HTML tag and silently drop it, corrupting the command an
// operator would copy into a CAB ticket.
func TestWriteHTML_EscapesPlaceholderSyntax(t *testing.T) {
	rpt := sampleReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "&lt;cluster&gt;") {
		t.Errorf("HTML output does not contain escaped &lt;cluster&gt; — placeholder syntax was not properly escaped")
	}
	if strings.Contains(out, "<cluster>") {
		t.Errorf("HTML output contains a raw, unescaped <cluster> tag — this would be silently dropped by a browser")
	}
}

func TestWriteHTML_ContainsExpectedSections(t *testing.T) {
	rpt := sampleReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"<!DOCTYPE html>",
		"KubePreflight Scan Report",
		`class="badge blocked"`,
		"Blockers (1)",
		"Warnings (1)",
		"Next Actions (1)",
		"Evidence Appendix",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}
}

func TestWriteTerminal_CleanReportShowsNoSections(t *testing.T) {
	rpt := findings.NewReport("1.34", "clean-cluster", "", time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC), nil)
	var buf bytes.Buffer
	if err := WriteTerminal(rpt, &buf); err != nil {
		t.Fatalf("WriteTerminal: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "Result: CLEAN") {
		t.Errorf("expected CLEAN result, got: %s", out)
	}
	if strings.Contains(out, "Blockers") || strings.Contains(out, "Warnings") || strings.Contains(out, "Next Actions") {
		t.Errorf("clean report must not print empty section headers, got: %s", out)
	}
}
