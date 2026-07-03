package report

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"kubepreflight/internal/findings"
)

func sampleReport() *findings.Report {
	fs := []findings.Finding{
		{
			RuleID: "WH-002", Severity: findings.SeverityBlocker, Confidence: findings.TierStaticCertain,
			Message:   `webhook "payments-guard" is fail-closed with no ready endpoints`,
			Resources: []findings.ResourceReference{findings.LiveResource("ValidatingWebhookConfiguration", findings.ScopeCluster, "", "payments-guard", "uid-1")},
			Evidence:  []string{"webhook index: 0", "ready endpoint address count: 0"},
			// Deliberately includes placeholder syntax like a real
			// remediation would (e.g. ADDON-001/API-001's `<cluster>`,
			// `<file>`) to exercise HTML escaping.
			Remediation: "Run: aws eks update-addon --cluster-name <cluster> --addon-name vpc-cni",
			Fingerprint: "fp-wh002",
		},
		{
			RuleID: "WH-001", Severity: findings.SeverityWarning, Confidence: findings.TierStaticCertain,
			Message:     `webhook "payments-guard" has catch-all scope`,
			Resources:   []findings.ResourceReference{findings.LiveResource("ValidatingWebhookConfiguration", findings.ScopeCluster, "", "payments-guard", "uid-1")},
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

// TestWriteHTML_HasCollapsibleEvidenceAndRemediation guards the demo-ready
// polish: Blockers/Warnings evidence and remediation must be collapsed by
// default (native <details>, no `open` attribute) so a long report doesn't
// read as a wall of text — while Next Actions remediation stays visible
// since that section IS the primary actionable summary.
func TestWriteHTML_HasCollapsibleEvidenceAndRemediation(t *testing.T) {
	rpt := sampleReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "<details><summary>Evidence") {
		t.Errorf("HTML output missing collapsible Evidence <details> block:\n%s", out)
	}
	if !strings.Contains(out, "<details><summary>Remediation") {
		t.Errorf("HTML output missing collapsible Remediation <details> block:\n%s", out)
	}
	if strings.Contains(out, `<details open>`) {
		t.Errorf("HTML output has a <details open> block — evidence/remediation must be collapsed by default")
	}
}

// TestWriteHTML_HasFilterToolbarAndDataAttributes guards the vanilla-JS
// filter/search pass: severity checkboxes, rule-ID and resource-name text
// inputs, and matching data-* attributes on every filterable row so the
// inline script has something to filter against.
func TestWriteHTML_HasFilterToolbarAndDataAttributes(t *testing.T) {
	rpt := sampleReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		`class="sev-filter" value="Blocker"`,
		`class="sev-filter" value="Warning"`,
		`class="sev-filter" value="Info"`,
		`id="rule-filter"`,
		`id="resource-filter"`,
		`id="filter-count"`,
		`data-severity="Blocker" data-rule-ids="WH-002"`,
		`data-severity="Warning" data-rule-ids="WH-001"`,
		"<script>",
		"addEventListener",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}
}

// TestWriteHTML_IsSingleSelfContainedFile guards the explicit "not a
// dashboard" constraint: no external stylesheet/script references, no CDN
// links — everything inline in one file.
func TestWriteHTML_IsSingleSelfContainedFile(t *testing.T) {
	rpt := sampleReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, unwanted := range []string{"<link ", "src=\"http", "href=\"http"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("HTML output contains external reference %q — report.html must stay a single self-contained file", unwanted)
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

func TestCrossPlaneAssumptionAppearsInEveryHumanReport(t *testing.T) {
	live := findings.LiveResource("Deployment", findings.ScopeNamespaced, "payments", "api", "uid-api")
	manifest := findings.ManifestResource("Deployment", findings.ScopeNamespaced, "payments", "api", "deploy/api.yaml")
	f := findings.Finding{
		RuleID: "API-001", Severity: findings.SeverityBlocker, Confidence: findings.TierStaticCertain,
		Message: "deprecated API", Resources: []findings.ResourceReference{live, manifest},
		Fingerprint: findings.FingerprintV2("API-001", "1.34", "", live, manifest),
	}
	rpt := findings.NewReport("1.34", "prod", "", time.Now(), []findings.Finding{f})
	if len(rpt.Assumptions) != 1 {
		t.Fatalf("report assumptions = %v, want cross-plane assumption", rpt.Assumptions)
	}

	writers := []struct {
		name string
		fn   func(*findings.Report, io.Writer) error
	}{{"terminal", WriteTerminal}, {"markdown", WriteMarkdown}, {"html", WriteHTML}}
	for _, writer := range writers {
		t.Run(writer.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := writer.fn(rpt, &buf); err != nil {
				t.Fatalf("rendering: %v", err)
			}
			if !strings.Contains(buf.String(), findings.CrossPlaneManifestAssumption) {
				t.Errorf("report missing cross-plane assumption:\n%s", buf.String())
			}
		})
	}
}

func TestNamespaceAllowlistAppearsInEveryReport(t *testing.T) {
	rpt := sampleReport()
	rpt.NamespaceAllowlist = []string{"payments", "platform"}

	var jsonBuf bytes.Buffer
	if err := WriteJSON(rpt, &jsonBuf); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if !strings.Contains(jsonBuf.String(), `"namespaceAllowlist"`) {
		t.Errorf("JSON missing namespace allowlist: %s", jsonBuf.String())
	}

	writers := []struct {
		name string
		fn   func(*findings.Report, io.Writer) error
	}{{"terminal", WriteTerminal}, {"markdown", WriteMarkdown}, {"html", WriteHTML}}
	for _, writer := range writers {
		t.Run(writer.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := writer.fn(rpt, &buf); err != nil {
				t.Fatalf("rendering: %v", err)
			}
			if !strings.Contains(buf.String(), "payments") || !strings.Contains(buf.String(), "platform") {
				t.Errorf("report missing active namespace allowlist: %s", buf.String())
			}
		})
	}
}
