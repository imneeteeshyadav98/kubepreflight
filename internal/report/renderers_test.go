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

// TestWriteCompactSummary_OmitsFindingDetail is the core guard for the
// --terminal-output=compact contract: report.html and the Console already
// carry every finding's evidence and remediation, so the compact summary
// used when the local report server starts must not repeat any of it —
// only cluster/target/provider/result/counts, matching what WriteTerminal
// prints in "full" mode (still exercised, unchanged, by
// TestWriteTerminal_ContainsExpectedSections above).
func TestWriteCompactSummary_OmitsFindingDetail(t *testing.T) {
	rpt := sampleReport()
	var buf bytes.Buffer
	if err := WriteCompactSummary(rpt, &buf); err != nil {
		t.Fatalf("WriteCompactSummary: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"prod-cluster", // cluster
		"1.34",         // target version
		"eks",          // provider
		"Result: BLOCKED",
		"Blockers: 1",
		"Warnings: 1",
		"Info: 0",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("compact summary missing %q\n--- full output ---\n%s", want, out)
		}
	}

	for _, unwanted := range []string{
		"Evidence:",
		"Remediation:",
		"Next Actions",
		"Also see",
		"[WH-001]",
		"[WH-002]",
		"webhook index: 0", // WH-002's evidence text — must not leak through
	} {
		if strings.Contains(out, unwanted) {
			t.Errorf("compact summary contains %q, want finding detail omitted entirely\n--- full output ---\n%s", unwanted, out)
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

// TestWriteHTML_FindingRowsAreCollapsedCompactRows guards the UI-polish
// pass: each Blockers/Warnings finding is one compact <details> row (badges
// + resource + message on the summary line), collapsed by default, with
// Evidence/Remediation inside — not the earlier design's two separate
// per-finding <details> blocks, and not always-expanded bulky cards.
func TestWriteHTML_FindingRowsAreCollapsedCompactRows(t *testing.T) {
	rpt := sampleReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, `<details class="finding-row blocker"`) {
		t.Errorf("HTML output missing a collapsible finding-row for blockers:\n%s", out)
	}
	if !strings.Contains(out, `<details class="finding-row warning"`) {
		t.Errorf("HTML output missing a collapsible finding-row for warnings:\n%s", out)
	}
	if !strings.Contains(out, "<h4>Evidence</h4>") || !strings.Contains(out, "<h4>Remediation</h4>") {
		t.Errorf("HTML output missing Evidence/Remediation headings inside a finding row:\n%s", out)
	}
	if strings.Contains(out, `<details open`) {
		t.Errorf("HTML output has a <details open> block — findings must be collapsed by default")
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

// TestWriteHTML_FilterCountMatchesActualFindingCount guards a real bug
// found during a demo run: the toolbar's "Showing X of Y findings" counter
// was computed from every element carrying data-severity, which includes
// Blockers/Warnings cards AND Next Actions items AND Evidence Appendix
// rows — so a report with 2 real findings displayed "Showing 20 of 20"
// instead of "2 of 2". Only elements marked data-finding="true" (the
// Blockers/Warnings cards, which are exactly 1:1 with Summary's
// blocker+warning counts) may be counted; Next Actions/Appendix rows must
// stay filterable (data-severity) without being counted twice.
func TestWriteHTML_FilterCountMatchesActualFindingCount(t *testing.T) {
	rpt := sampleReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	wantFindings := len(rpt.Findings)
	gotFindingMarkers := strings.Count(out, `data-finding="true"`)
	if gotFindingMarkers != wantFindings {
		t.Errorf("data-finding markers = %d, want %d (must match actual finding count, not sections)", gotFindingMarkers, wantFindings)
	}

	// The JS counter must be scored against data-finding elements, not
	// every data-severity element (which over-counts across sections).
	if !strings.Contains(out, "findingRows.length") {
		t.Errorf("HTML filter script does not scope its count to data-finding elements:\n%s", out)
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

// TestWriteHTML_HasExecutiveHeaderAndCards guards the demo-readiness polish:
// a navy banner header with result/cluster/target/provider/AWS-enrichment/
// scanned-at, summary metric cards, a confidence-mix panel, tabbed section
// nav, and per-finding copy-remediation buttons — bringing report.html to
// the same visual language as the Console (web/) instead of reading as a
// bare developer dump next to it.
func TestWriteHTML_HasExecutiveHeaderAndCards(t *testing.T) {
	rpt := sampleReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		`class="banner"`,
		`class="summary-grid"`,
		`class="metric metric-blocker"`,
		`class="metric metric-warning"`,
		`class="confidence-panel"`,
		`class="confidence-stat"`,
		`class="tab-nav screen-only"`,
		`data-tab="findings"`,
		`data-tab="actions"`,
		`data-tab="evidence"`,
		`AWS enrichment`,
		`class="copy-btn"`,
		"Copy remediation",
		`class="console-link screen-only"`,
		`href="/console/?findings=/findings.json#summary"`,
		"Open Interactive Console",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}
}

// TestWriteHTML_IsSinglePageWithTabs guards the command-center pass: only
// the Summary tab panel is visible by default (BLockers/Warnings/Next
// Actions/Evidence Appendix all render, but behind hidden tab panels), and
// printing must reveal everything — a physical CAB packet has no tabs.
func TestWriteHTML_IsSinglePageWithTabs(t *testing.T) {
	rpt := sampleReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		`<div class="tab-panel" data-panel="summary"`,
		`<div class="tab-panel hidden" data-panel="findings"`,
		`<div class="tab-panel hidden" data-panel="actions"`,
		`<div class="tab-panel hidden" data-panel="evidence"`,
		"beforeprint",
		"afterprint",
		"@media print",
		".screen-only { display: none !important; }",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}

	// Summary tab is a preview only — Top next actions, not the full list.
	if !strings.Contains(out, "Top next actions") {
		t.Errorf("HTML output missing the Summary tab's next-actions preview")
	}
}

// TestWriteHTML_HasDecisionAndTopRisks guards the executive-summary polish:
// a GO/REVIEW/NO-GO decision label and why-line above the fold, and a Top
// Risks strip surfacing the highest-severity findings first — so a CAB
// reviewer doesn't have to scroll past a wall of cards to find out what's
// actually blocking the upgrade.
func TestWriteHTML_HasDecisionAndTopRisks(t *testing.T) {
	rpt := sampleReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		`class="decision-mark blocked"`,
		`class="decision-label"`,
		"NO-GO",
		"1 blocker found — fix required before the change window.",
		`id="top-risks"`,
		"Top risks",
		`class="rank"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}

	// The blocker (WH-002) must be listed before the warning (WH-001) in
	// Top Risks — highest severity first.
	blockerIdx := strings.Index(out, `<span class="rule-id">WH-002</span>`)
	warningIdx := strings.Index(out, `<span class="rule-id">WH-001</span>`)
	// The first WH-002/WH-001 rule-id occurrence in the document is inside
	// Top Risks (it renders before Next Actions/Findings), so a simple
	// ordering check on the first match is sufficient here.
	if blockerIdx == -1 || warningIdx == -1 || blockerIdx > warningIdx {
		t.Errorf("expected the blocker (WH-002) to appear before the warning (WH-001) in Top Risks")
	}
}

func TestDecisionLabel(t *testing.T) {
	cases := map[string]string{"BLOCKED": "NO-GO", "PASSED_WITH_WARNINGS": "REVIEW", "CLEAN": "GO"}
	for result, want := range cases {
		if got := decisionLabel(result); got != want {
			t.Errorf("decisionLabel(%q) = %q, want %q", result, got, want)
		}
	}
}

func TestDecisionWhyLine(t *testing.T) {
	cases := []struct {
		blockers, warnings int
		want               string
	}{
		{2, 0, "2 blockers found — fix required before the change window."},
		{1, 0, "1 blocker found — fix required before the change window."},
		{0, 3, "3 warnings found — review before the change window."},
		{0, 1, "1 warning found — review before the change window."},
		{0, 0, "No blockers or warnings — safe to proceed."},
	}
	for _, c := range cases {
		if got := decisionWhyLine(c.blockers, c.warnings); got != c.want {
			t.Errorf("decisionWhyLine(%d, %d) = %q, want %q", c.blockers, c.warnings, got, c.want)
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

func TestWriteHTML_RendersChangeRequiredAndCopyButtons(t *testing.T) {
	fs := []findings.Finding{
		{
			RuleID: "API-001", Severity: findings.SeverityBlocker, Confidence: findings.TierStaticCertain,
			Message:     "PodDisruptionBudget uses a removed apiVersion",
			Resources:   []findings.ResourceReference{findings.ManifestResource("PodDisruptionBudget", findings.ScopeNamespaced, "prod-like", "old-pdb-api", "manifests/old-pdb-api.yaml")},
			Remediation: "Migrate to policy/v1.",
			RemediationDetail: &findings.RemediationDetail{
				AffectedFile:  "manifests/old-pdb-api.yaml",
				Changes:       []findings.RemediationChange{{Field: "apiVersion", Current: "policy/v1beta1", Required: "policy/v1"}},
				Diff:          "- apiVersion: policy/v1beta1\n+ apiVersion: policy/v1",
				SafeFix:       &findings.RemediationAction{Label: "Safe fix", Command: "kubectl convert -f <file> --output-version policy/v1"},
				VerifyCommand: "kubepreflight scan --manifests manifests --target-version 1.36",
			},
			Fingerprint: "fp-api001",
		},
	}
	rpt := findings.NewReport("1.36", "prod-like", "", time.Now(), fs)
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		`class="change-required"`,
		"Change required",
		"policy/v1beta1",
		`class="change-arrow"`,
		"Suggested diff",
		"Copy diff",
		"Safe fix",
		"Copy command",
		"Copy verify command",
		`data-copy-target="blocker-0-diff"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}
}

// TestWriteHTML_NextActionsRendersGroupedPlanForMergedFindings is the
// user-worked scenario: PDB-001 fires on payment-api-pdb, PDB-002 fires on
// (payment-api-pdb, payment-api-pdb-duplicate). They merge into one Next
// Action (see TestBuildNextActions_MergesAcrossPartiallyOverlappingResources),
// and the HTML report must render a grouped, numbered remediation plan for
// it rather than only the primary finding's remediation text.
func TestWriteHTML_NextActionsRendersGroupedPlanForMergedFindings(t *testing.T) {
	pdb := []findings.ResourceReference{findings.LiveResource("PodDisruptionBudget", findings.ScopeNamespaced, "prod-like", "payment-api-pdb", "uid-payment-api-pdb")}
	duplicate := findings.LiveResource("PodDisruptionBudget", findings.ScopeNamespaced, "prod-like", "payment-api-pdb-duplicate", "uid-payment-api-pdb-duplicate")

	fs := []findings.Finding{
		{
			RuleID: "PDB-001", Severity: findings.SeverityBlocker, Confidence: findings.TierStaticCertain,
			Message:     "disruptionsAllowed=0",
			Resources:   pdb,
			Remediation: "scale up replicas",
			RemediationDetail: &findings.RemediationDetail{
				SafeFix: &findings.RemediationAction{Label: "Safe fix", Command: "kubectl scale deployment <workload-name> -n prod-like --replicas=<N>"},
			},
			Fingerprint: "fp-pdb001",
		},
		{
			RuleID: "PDB-002", Severity: findings.SeverityBlocker, Confidence: findings.TierStaticCertain,
			Message:     "overlapping PDBs",
			Resources:   append(append([]findings.ResourceReference{}, pdb...), duplicate),
			Remediation: "delete the duplicate PDB",
			RemediationDetail: &findings.RemediationDetail{
				SafeFix: &findings.RemediationAction{Label: "Safe fix", Command: "kubectl delete pdb payment-api-pdb-duplicate -n prod-like"},
			},
			Fingerprint: "fp-pdb002",
		},
	}
	rpt := findings.NewReport("1.36", "prod-like", "", time.Now(), fs)
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, `class="grouped-plan"`) {
		t.Fatalf("HTML output missing grouped-plan ordered list for the merged PDB-001/PDB-002 action")
	}
	for _, want := range []string{"[PDB-001]", "[PDB-002]", "kubectl scale deployment", "kubectl delete pdb"} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q in grouped plan", want)
		}
	}
}

func globalBlockerReport() *findings.Report {
	fs := []findings.Finding{
		{
			RuleID: "WH-002", Severity: findings.SeverityBlocker, Confidence: findings.TierStaticCertain,
			Message:     "webhook is fail-closed with zero endpoints",
			Resources:   []findings.ResourceReference{findings.LiveResource("ValidatingWebhookConfiguration", findings.ScopeCluster, "", "catch-all-guard", "uid-webhook")},
			Remediation: "restore backend health",
			RemediationDetail: &findings.RemediationDetail{
				SafeFix: &findings.RemediationAction{Label: "Safe fix", Command: "kubectl get svc guard-svc -n guard-ns"},
			},
			GlobalBlocker: true,
			Fingerprint:   "fp-wh002",
		},
		{
			RuleID: "PDB-001", Severity: findings.SeverityBlocker, Confidence: findings.TierStaticCertain,
			Message:     "disruptionsAllowed=0",
			Resources:   []findings.ResourceReference{findings.LiveResource("PodDisruptionBudget", findings.ScopeNamespaced, "prod-like", "payment-api-pdb", "uid-pdb")},
			Remediation: "scale up replicas",
			RemediationDetail: &findings.RemediationDetail{
				SafeFix: &findings.RemediationAction{Label: "Safe fix", Command: "kubectl scale deployment payment-api -n prod-like --replicas=2"},
			},
			Fingerprint: "fp-pdb001",
		},
		{
			RuleID: "API-001", Severity: findings.SeverityBlocker, Confidence: findings.TierStaticCertain,
			Message:     "deprecated apiVersion in manifest",
			Resources:   []findings.ResourceReference{findings.ManifestResource("PodDisruptionBudget", findings.ScopeNamespaced, "prod-like", "old-pdb-api", "manifests/old-pdb-api.yaml")},
			Remediation: "migrate to policy/v1",
			Fingerprint: "fp-api001",
		},
	}
	return findings.NewReport("1.36", "prod-like", "", time.Now(), fs)
}

func TestWriteHTML_GlobalBlockerBannerBadgeAndDependencyWarning(t *testing.T) {
	rpt := globalBlockerReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		`class="global-blocker-banner"`,
		"Global API write blocker detected",
		"1 global blocker may prevent remediation commands from running.",
		`class="global-blocker-badge"`,
		"GLOBAL API WRITE BLOCKER",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}

	// The PDB-001 card (a live-plane finding with a remediation command,
	// not itself the global blocker) must show the dependency warning.
	pdbCardStart := strings.Index(out, `data-rule-ids="PDB-001"`)
	pdbCardEnd := strings.Index(out[pdbCardStart:], "</details>") + pdbCardStart
	if pdbCardStart < 0 || !strings.Contains(out[pdbCardStart:pdbCardEnd], "This command may fail until the admission webhook blocker is fixed.") {
		t.Errorf("PDB-001 card missing the dependency warning")
	}

	// The webhook's own card must never show the dependency warning about
	// itself.
	whCardStart := strings.Index(out, `data-rule-ids="WH-002"`)
	whCardEnd := strings.Index(out[whCardStart:], "</details>") + whCardStart
	if whCardStart < 0 || strings.Contains(out[whCardStart:whCardEnd], "This command may fail until the admission webhook blocker is fixed.") {
		t.Errorf("WH-002's own card must not show the dependency warning about itself")
	}

	// API-001 is manifest-only (no live resource) — editing a local YAML
	// file isn't blocked by a cluster-side admission webhook.
	apiCardStart := strings.Index(out, `data-rule-ids="API-001"`)
	apiCardEnd := strings.Index(out[apiCardStart:], "</details>") + apiCardStart
	if apiCardStart < 0 || strings.Contains(out[apiCardStart:apiCardEnd], "This command may fail until the admission webhook blocker is fixed.") {
		t.Errorf("API-001 (manifest-only) card must not show the dependency warning")
	}
}

func TestWriteHTML_NoGlobalBlockerHidesBanner(t *testing.T) {
	rpt := sampleReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, `class="global-blocker-banner"`) {
		t.Errorf("HTML output has a global-blocker-banner despite no GlobalBlocker findings")
	}
}

// TestWriteHTML_TopRisksOrdersGlobalBlockerFirst proves the global-blocker
// tiebreak in topRisks isn't just coincidental: WH-002 and PDB-001/API-001
// are all Blocker severity, and "WH-002" doesn't naturally sort first by
// rule ID — it must still lead because it's the global blocker.
func TestWriteHTML_TopRisksOrdersGlobalBlockerFirst(t *testing.T) {
	rpt := globalBlockerReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	risksStart := strings.Index(out, `class="top-risks-list"`)
	if risksStart < 0 {
		t.Fatalf("top-risks-list not found in output")
	}
	firstRiskRuleIDPos := strings.Index(out[risksStart:], `class="rule-id">WH-002<`)
	firstOtherRuleIDPos := strings.Index(out[risksStart:], `class="rule-id">API-001<`)
	if firstRiskRuleIDPos < 0 || firstOtherRuleIDPos < 0 || firstRiskRuleIDPos > firstOtherRuleIDPos {
		t.Errorf("WH-002 (global blocker) must appear before API-001 in Top Risks; WH-002 at %d, API-001 at %d", firstRiskRuleIDPos, firstOtherRuleIDPos)
	}
}
