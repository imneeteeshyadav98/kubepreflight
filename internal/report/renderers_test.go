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
	rpt := findings.NewReport("1.34", "prod-cluster", "eks", time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC), fs)
	rpt.CurrentVersion = "1.33"
	return rpt
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
	if decoded.SchemaVersion != findings.SchemaVersion || decoded.Coverage.Kubernetes.Status != findings.CoverageComplete {
		t.Errorf("schema/coverage contract missing: %+v", decoded)
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
		"/WH-002]",
		"Warnings (1)",
		"/WH-001]",
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
		`class="metric metric-blocker metric-button"`,
		`class="metric metric-warning metric-button"`,
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

func TestWriteHTML_SummaryCountCardsNavigateWhenNonZero(t *testing.T) {
	rpt := sampleReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		`type="button" class="metric metric-blocker metric-button" data-goto-severity="Blocker" aria-label="View blocker findings"`,
		`type="button" class="metric metric-warning metric-button" data-goto-severity="Warning" aria-label="View warning findings"`,
		`View blocker findings`,
		`View warning findings`,
		`document.querySelectorAll('[data-goto-severity]')`,
		`activateTab('findings')`,
		`b.checked = b.value === severity`,
		`ruleInput.value = ''`,
		`resourceInput.value = ''`,
		`document.querySelector('[data-finding][data-severity="' + severity + '"]')`,
		`highlightAndScroll(target)`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}
}

func TestWriteHTML_SummaryCountCardsDisabledWhenZero(t *testing.T) {
	rpt := findings.NewReport("1.34", "prod-cluster", "eks", time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC), []findings.Finding{
		{
			RuleID: "WH-002", Severity: findings.SeverityBlocker, Confidence: findings.TierStaticCertain,
			Message:     `webhook "payments-guard" is fail-closed with no ready endpoints`,
			Resources:   []findings.ResourceReference{findings.LiveResource("ValidatingWebhookConfiguration", findings.ScopeCluster, "", "payments-guard", "uid-1")},
			Evidence:    []string{"webhook index: 0", "ready endpoint address count: 0"},
			Remediation: "Run: aws eks update-addon --cluster-name <cluster> --addon-name vpc-cni",
			Fingerprint: "fp-wh002",
		},
	})
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		`type="button" class="metric metric-blocker metric-button" data-goto-severity="Blocker" aria-label="View blocker findings"`,
		`<article class="metric metric-warning" aria-disabled="true"><span>Warnings</span><strong>0</strong><small>No warnings found</small></article>`,
		`<article class="metric metric-info" aria-disabled="true"><span>Info</span><strong>0</strong><small>No info findings</small></article>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}
	for _, unwanted := range []string{
		`data-goto-severity="Warning" aria-label="View warning findings"`,
		`data-goto-severity="Info" aria-label="View info findings"`,
	} {
		if strings.Contains(out, unwanted) {
			t.Errorf("zero-count card should not be clickable, found %q", unwanted)
		}
	}
}

func TestWriteHTML_RendersUpgradeContextWhenCurrentVersionPresent(t *testing.T) {
	rpt := sampleReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		`<dt>Current version</dt><dd>1.33</dd>`,
		`<dt>Target version</dt><dd>1.34</dd>`,
		`1.33 → 1.34`,
		`one-minor upgrade`,
		`This scan checks readiness for upgrading from 1.33 to 1.34.`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}
}

func TestWriteHTML_RendersUnknownCurrentVersionWhenAbsent(t *testing.T) {
	rpt := sampleReport()
	rpt.CurrentVersion = ""
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		`<dt>Current version</dt><dd>Unknown</dd>`,
		`Unknown → 1.34`,
		`current version unknown`,
		`Node/kubelet versions are evaluated separately.`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}
}

func TestWriteHTML_RendersMultiMinorUpgradePath(t *testing.T) {
	rpt := sampleReport()
	rpt.CurrentVersion = "v1.32.2"
	rpt.TargetVersion = "1.36"
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		`1.32 → 1.33 → 1.34 → 1.35 → 1.36`,
		`multi-minor upgrade path`,
		`This scan checks readiness for upgrading from 1.32 to 1.36.`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}
}

func TestWriteHTML_RendersUpgradePathDetailsForSingleHop(t *testing.T) {
	rpt := sampleReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		`<h2 class="section-title">Upgrade path details</h2>`,
		`<span class="hop-versions">1.33 &rarr; 1.34</span>`,
		`<span class="badge-blocked">Blocked</span>`,
		`Current findings must be resolved before this hop should proceed.`,
		`Admission webhooks: 1 blocker(s), 1 warning(s) (WH-001, WH-002)`,
		`API removals and deprecated API usage`,
		`Release notes review for the target minor`,
		`Re-scan after each hop before treating the next hop as assessed.`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}
	if strings.Contains(out, "Planned, re-scan required") {
		t.Error("single-hop upgrade should not render future-hop rescan status")
	}
}

func TestWriteHTML_RendersUpgradePathDetailsForFutureHops(t *testing.T) {
	rpt := sampleReport()
	rpt.CurrentVersion = "1.32"
	rpt.TargetVersion = "1.36"
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		`1 fix required before upgrading to 1.36`,
		`<span class="hop-versions">1.32 &rarr; 1.33</span>`,
		`<span class="hop-versions">1.33 &rarr; 1.34</span>`,
		`<span class="hop-versions">1.34 &rarr; 1.35</span>`,
		`<span class="hop-versions">1.35 &rarr; 1.36</span>`,
		`Planned, hop-specific scan recommended`,
		`Planned, re-scan required`,
		`Findings were evaluated against final target 1.36, not this individual hop.`,
		`Overall target blockers remain listed in this report, but they are not proof that this intermediate hop is blocked.`,
		`Do not treat this future hop as safe yet.`,
		`Findings were evaluated against final target 1.36; current findings are not projected as proof for this future cluster state.`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}
	for _, notWant := range []string{
		`<span class="badge-blocked">Blocked</span>`,
		`Current findings must be resolved before this hop should proceed.`,
		`Admission webhooks: 1 blocker(s), 1 warning(s) (WH-001, WH-002)`,
	} {
		if strings.Contains(out, notWant) {
			t.Errorf("multi-hop HTML should not project final-target findings onto the first hop; found %q", notWant)
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
		`<div class="tab-panel" role="tabpanel" data-panel="summary"`,
		`<div class="tab-panel hidden" role="tabpanel" data-panel="findings"`,
		`<div class="tab-panel hidden" role="tabpanel" data-panel="actions"`,
		`<div class="tab-panel hidden" role="tabpanel" data-panel="evidence"`,
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

// TestWriteHTML_GroupedPlanPreservesMultiCommandNewlines guards a real
// UX regression found via a live kind-cluster smoke test: PDB-001's
// SafeFix.Command is two kubectl commands joined by a literal "\n" (see
// pdb001.go), and groupedPlanStep embeds that whole string into a plain
// <li> (internal/report/html.go's "ol.grouped-plan" template). A plain
// HTML element collapses embedded newlines when rendered in a browser —
// unlike a <pre> block — which visually concatenates the two commands
// into one run-on line with no separator, looking like a single copyable
// command that would actually fail if pasted verbatim. The HTML source
// keeps the real newline (confirmed below), so the fix is CSS-only:
// "ol.grouped-plan li" must set white-space: pre-wrap (matching how the
// base "pre" rule already treats every other multi-line command block in
// this report) so the newline still renders as a line break.
func TestWriteHTML_GroupedPlanPreservesMultiCommandNewlines(t *testing.T) {
	pdb := []findings.ResourceReference{findings.LiveResource("PodDisruptionBudget", findings.ScopeNamespaced, "preflight-lab", "demo-app-pdb", "uid-demo-app-pdb")}
	duplicate := findings.LiveResource("PodDisruptionBudget", findings.ScopeNamespaced, "preflight-lab", "demo-app-pdb-duplicate", "uid-demo-app-pdb-duplicate")

	fs := []findings.Finding{
		{
			RuleID: "PDB-001", Severity: findings.SeverityBlocker, Confidence: findings.TierObserved,
			Message: "disruptionsAllowed=0", Resources: pdb, Remediation: "scale up replicas",
			RemediationDetail: &findings.RemediationDetail{
				SafeFix: &findings.RemediationAction{
					Label:   "Safe fix",
					Command: "kubectl get pdb demo-app-pdb -n preflight-lab -o yaml\nkubectl get pods -n preflight-lab --show-labels",
				},
			},
			Fingerprint: "fp-pdb001",
		},
		{
			RuleID: "PDB-002", Severity: findings.SeverityBlocker, Confidence: findings.TierObserved,
			Message: "overlapping PDBs", Resources: append(append([]findings.ResourceReference{}, pdb...), duplicate), Remediation: "inspect both budgets",
			RemediationDetail: &findings.RemediationDetail{
				SafeFix: &findings.RemediationAction{Label: "Safe fix", Command: "kubectl get pdb demo-app-pdb demo-app-pdb-duplicate -n preflight-lab -o yaml"},
			},
			Fingerprint: "fp-pdb002",
		},
	}
	rpt := findings.NewReport("1.36", "preflight-lab", "", time.Now(), fs)
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "kubectl get pdb demo-app-pdb -n preflight-lab -o yaml\nkubectl get pods -n preflight-lab --show-labels") {
		t.Fatalf("grouped-plan <li> must keep the real newline between the two commands, got: %s", out)
	}
	if strings.Contains(out, "-o yaml kubectl get pods") {
		t.Fatalf("the two commands must never be collapsed onto one line with no separator")
	}
	idx := strings.Index(out, "ol.grouped-plan li")
	if idx == -1 {
		t.Fatal("missing \"ol.grouped-plan li\" CSS rule")
	}
	end := strings.Index(out[idx:], "}")
	if end == -1 {
		t.Fatal("unterminated \"ol.grouped-plan li\" CSS rule")
	}
	rule := out[idx : idx+end]
	if !strings.Contains(rule, "white-space: pre-wrap") {
		t.Fatalf("ol.grouped-plan li rule = %q, must set white-space: pre-wrap so the browser doesn't collapse the embedded newline", rule)
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

func TestIncompleteCoverageAppearsInHumanReports(t *testing.T) {
	rpt := findings.NewReport("1.34", "prod", "", time.Now(), nil)
	rpt.Coverage.Kubernetes = findings.PlaneCoverage{Status: findings.CoveragePartial, Errors: []string{"pods: forbidden"}}
	for name, write := range map[string]func(*findings.Report, io.Writer) error{"terminal": WriteTerminal, "markdown": WriteMarkdown, "html": WriteHTML} {
		var buf bytes.Buffer
		if err := write(rpt, &buf); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !strings.Contains(strings.ToLower(buf.String()), "assessment incomplete") || !strings.Contains(buf.String(), "pods: forbidden") {
			t.Fatalf("%s omitted coverage warning: %s", name, buf.String())
		}
	}
}

func TestWriteHTML_ContainsClipboardFallbackAndAccessibleTabs(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteHTML(sampleReport(), &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"fallbackCopy", `role="tab"`, `aria-selected="true"`, "ArrowRight"} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML missing %q", want)
		}
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

// TestHeroCopy_CoversEveryResult guards the operator-clarity hero framing
// added alongside the technical GO/REVIEW/NO-GO badge: every Result value
// gets a plain-English title/subtext/explanation, and the subtext cites
// the actual blocker/warning count and target version rather than static
// text (which the audit's UX pass specifically asked for: "operator can
// understand in 10 seconds").
func TestHeroCopy_CoversEveryResult(t *testing.T) {
	cases := []struct {
		result           string
		blockers         int
		warnings         int
		wantTitle        string
		wantSubtextParts []string
	}{
		{"BLOCKED", 2, 0, "Upgrade blocked", []string{"2", "fixes", "1.36"}},
		{"BLOCKED", 1, 0, "Upgrade blocked", []string{"1 fix", "1.36"}},
		{"PASSED_WITH_WARNINGS", 0, 3, "Upgrade needs review", []string{"3", "items", "1.36"}},
		{"PASSED_WITH_WARNINGS", 0, 1, "Upgrade needs review", []string{"1 item", "1.36"}},
		{"INCOMPLETE", 0, 0, "Assessment incomplete", []string{"1.36"}},
		{"CLEAN", 0, 0, "Ready to upgrade", []string{"1.36"}},
	}
	for _, c := range cases {
		title, subtext, explain := heroCopy(c.result, c.blockers, c.warnings, "1.36")
		if title != c.wantTitle {
			t.Errorf("heroCopy(%q, %d, %d) title = %q, want %q", c.result, c.blockers, c.warnings, title, c.wantTitle)
		}
		for _, part := range c.wantSubtextParts {
			if !strings.Contains(subtext, part) {
				t.Errorf("heroCopy(%q, %d, %d) subtext = %q, want it to contain %q", c.result, c.blockers, c.warnings, subtext, part)
			}
		}
		if explain == "" {
			t.Errorf("heroCopy(%q, %d, %d) explain is empty, want a plain-English sentence", c.result, c.blockers, c.warnings)
		}
	}
}

func TestSeverityActionLabel(t *testing.T) {
	cases := map[findings.Severity]string{
		findings.SeverityBlocker: "BLOCKER — must fix before upgrade",
		findings.SeverityWarning: "WARNING — review before upgrade",
		findings.SeverityInfo:    "INFO — no action required",
	}
	for sev, want := range cases {
		if got := severityActionLabel(sev); got != want {
			t.Errorf("severityActionLabel(%q) = %q, want %q", sev, got, want)
		}
	}
}

// TestRuleTitleAndWhy_FallBackForUnknownRuleID guards that a future rule
// added without a ruleCopyByID entry degrades gracefully (the bare rule
// ID as title, a generic explanation) instead of rendering an empty
// string or panicking.
func TestRuleTitleAndWhy_FallBackForUnknownRuleID(t *testing.T) {
	if got := ruleTitle("FUTURE-999"); got != "FUTURE-999" {
		t.Errorf("ruleTitle(unknown) = %q, want the bare rule ID as fallback", got)
	}
	if got := ruleWhy("FUTURE-999"); got == "" {
		t.Error("ruleWhy(unknown) is empty, want a generic fallback explanation")
	}
	// Every currently-registered rule ID must have real, non-empty copy —
	// guards against a typo'd map key silently falling back to the bare
	// ID for a rule that's supposed to have a friendly title.
	for _, ruleID := range []string{
		"API-001", "WH-001", "WH-002", "PDB-001", "PDB-002",
		"NODE-001", "NODE-002", "NET-002", "WORKLOAD-001", "ADDON-001",
		"EKS-NG-001", "EKS-NG-002", "EKS-NG-003", "EKS-NG-004",
		"EKS-INSIGHT-001", "EKS-INSIGHT-002", "EKS-INSIGHT-003", "COREDNS-001",
		"CRD-001", "CRD-002", "APISERVICE-001",
	} {
		if title := ruleTitle(ruleID); title == ruleID {
			t.Errorf("ruleTitle(%q) fell back to the bare rule ID — missing a ruleCopyByID entry", ruleID)
		}
		if why := ruleWhy(ruleID); why == "" || why == "This finding was flagged as a risk for the target upgrade version." {
			t.Errorf("ruleWhy(%q) fell back to the generic explanation — missing a ruleCopyByID entry", ruleID)
		}
	}
}

// TestWriteHTML_StartHereBoxReflectsBlockerOrder guards the new "Start
// here" guidance box: it must appear when there are actionable findings,
// list them in the same order as Next Actions (worst first), and tell
// the operator not to proceed until blockers = 0.
func TestWriteHTML_StartHereBoxReflectsBlockerOrder(t *testing.T) {
	rpt := sampleReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		`class="start-here"`,
		"Start here",
		"Fix these in order:",
		"Do not start the upgrade until blockers = 0.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}

	startHereIdx := strings.Index(out, `class="start-here"`)
	topRisksIdx := strings.Index(out, `id="top-risks"`)
	if startHereIdx < 0 || topRisksIdx < 0 || startHereIdx > topRisksIdx {
		t.Errorf("Start here box must render before Top Risks (guidance before detail); start-here at %d, top-risks at %d", startHereIdx, topRisksIdx)
	}
	upgradeDetailsIdx := strings.Index(out, `class="upgrade-path-details"`)
	if upgradeDetailsIdx < 0 || topRisksIdx > upgradeDetailsIdx {
		t.Errorf("Top Risks must render before Upgrade Path Details; top-risks at %d, upgrade-details at %d", topRisksIdx, upgradeDetailsIdx)
	}
	if !strings.Contains(out, `<summary>Show checks to review</summary>`) {
		t.Error("Upgrade Path Details must keep the repeated checklist behind a disclosure")
	}
}

// TestWriteHTML_NoActionableFindingsHidesStartHereBox guards that a clean
// report (no blockers/warnings) doesn't show an empty, meaningless
// "Start here" box.
func TestWriteHTML_NoActionableFindingsHidesStartHereBox(t *testing.T) {
	rpt := findings.NewReport("1.34", "clean-cluster", "", time.Now(), nil)
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	if out := buf.String(); strings.Contains(out, `class="start-here"`) {
		t.Error("HTML output includes a Start Here box for a clean report with nothing to fix")
	}
}

// TestWriteHTML_TopRisksCardsShowTitleWhyAndRuleChip guards the redesigned
// Top Risks cards: each one must show a plain-English title, a "why this
// blocks upgrade" explanation, a "Rule: X" chip (not just a bare rule
// code), and the original detailed Message still present (just under a
// collapsed <details> disclosure, not deleted) — the audit's UX pass
// explicitly asked to keep the original message available, not remove it.
func TestWriteHTML_TopRisksCardsShowTitleWhyAndRuleChip(t *testing.T) {
	rpt := sampleReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		`class="risk-card blocker"`,
		"Webhook backend is down", // ruleCopyByID["WH-002"].Title
		"Why this blocks upgrade",
		"What to do",
		"Rule: <span",             // the rule chip wraps the existing .rule-id span
		`class="rule-id">WH-002<`, // preserved exactly for the existing ordering tests
		"Show scan details",
		`webhook &#34;payments-guard&#34; is fail-closed with no ready endpoints`, // original Message, still present (html/template-escaped)
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}
}

// TestWriteHTML_NextActionsPreviewIsNotTruncated guards against the
// specific complaint the UX pass was about: the Summary tab's "Top next
// actions" preview must not clip remediation text (no line-clamp), must
// number the items (fix order), and must surface a real SafeFix command
// when the primary finding has one.
func TestWriteHTML_NextActionsPreviewIsNotTruncated(t *testing.T) {
	rpt := globalBlockerReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, `<ol class="preview-actions-list">`) {
		t.Error("Top next actions preview is not a numbered list")
	}
	if strings.Contains(out, "-webkit-line-clamp: 2") && strings.Contains(out, "preview-actions-list") {
		// The old truncation rule must not apply to the new preview
		// items; a global-blocker report's remediation text is short in
		// this fixture, so directly check the CSS rule for the class
		// that's actually applied to preview items no longer clamps.
		clampIdx := strings.Index(out, ".preview-actions-list .risk-reason")
		if clampIdx >= 0 && strings.Contains(out[clampIdx:clampIdx+200], "line-clamp") {
			t.Error(".preview-actions-list text is still clamped/truncated")
		}
	}
	if !strings.Contains(out, "kubectl get svc guard-svc -n guard-ns") {
		t.Error("Top next actions preview is missing the primary finding's real SafeFix command")
	}
}

// TestProviderDisplayLabel_KnownAndUnknownProviders guards the friendly
// metadata wording fix (RPT-004): known providers get their conventional
// casing (EKS, not "eks"), and an empty/cluster-only provider reads as
// "Cluster-only" instead of a raw internal enum value.
func TestProviderDisplayLabel_KnownAndUnknownProviders(t *testing.T) {
	cases := map[string]string{
		"eks":          "EKS",
		"aks":          "AKS",
		"gke":          "GKE",
		"cluster-only": "Cluster-only",
		"":             "Cluster-only",
		"custom":       "Custom",
	}
	for provider, want := range cases {
		if got := providerDisplayLabel(provider); got != want {
			t.Errorf("providerDisplayLabel(%q) = %q, want %q", provider, got, want)
		}
	}
}

func TestAWSEnrichmentLabel(t *testing.T) {
	if got := awsEnrichmentLabel(true); got != "On" {
		t.Errorf("awsEnrichmentLabel(true) = %q, want %q", got, "On")
	}
	if got := awsEnrichmentLabel(false); got != "Off" {
		t.Errorf("awsEnrichmentLabel(false) = %q, want %q", got, "Off")
	}
}

// TestWriteHTML_MetadataUsesFriendlyLabels guards that the rendered report
// never shows the raw Go bool ("true"/"false") or a bare lowercase
// provider enum ("eks") for operator-facing metadata — both the banner
// meta chips and the confidence panel's "scan source" row must use the
// friendly labels.
func TestWriteHTML_MetadataUsesFriendlyLabels(t *testing.T) {
	rpt := sampleReport() // provider "eks", AWS enrichment off (no AWS-plane findings)
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{"<dd>EKS</dd>", "<dd>Off</dd>", "Provider: EKS", "AWS enrichment: Off"} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}
	for _, unwanted := range []string{"<dd>eks</dd>", "<dd>false</dd>", "<dd>true</dd>"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("HTML output contains raw/unfriendly metadata value %q", unwanted)
		}
	}
}

// TestWriteHTML_SafeFixCommandsAreLabeledAsInspect guards the P1 finding
// from the operator-clarity audit: every rule's SafeFix.Command is a
// read-only kubectl get/describe (or aws ... describe-*) call, never an
// executable fix — the Findings tab's remediation panel, the Summary
// tab's Next Actions preview, and the Next Actions tab's grouped plan
// must all label it as an inspection step, not present it as if copying
// and running it resolves the finding.
func TestWriteHTML_SafeFixCommandsAreLabeledAsInspect(t *testing.T) {
	rpt := globalBlockerReport() // WH-002 (global blocker) + PDB-001, both with a SafeFix.Command
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "Inspect first") {
		t.Error("HTML output missing an \"Inspect first\" label near a SafeFix command")
	}
	if !strings.Contains(out, "does not change anything") {
		t.Error("HTML output missing the explicit \"does not change anything\" qualifier on an inspect command")
	}
}

// TestGroupedPlanStep_LabelsSafeFixCommandAsInspect is the grouped-plan
// (Next Actions tab, merged-finding case) counterpart of the same guard —
// a synthesized multi-step plan must not present step 1 as "the fix" when
// it's actually just gathering evidence.
func TestGroupedPlanStep_LabelsSafeFixCommandAsInspect(t *testing.T) {
	f := findings.Finding{
		RuleID: "PDB-001",
		RemediationDetail: &findings.RemediationDetail{
			SafeFix: &findings.RemediationAction{Command: "kubectl get pdb example -n ns -o yaml"},
		},
	}
	got := groupedPlanStep(f)
	if !strings.Contains(got, "Inspect:") {
		t.Errorf("groupedPlanStep(SafeFix.Command set) = %q, want it labeled with \"Inspect:\"", got)
	}
	if !strings.Contains(got, "kubectl get pdb example -n ns -o yaml") {
		t.Errorf("groupedPlanStep(SafeFix.Command set) = %q, want the actual command preserved", got)
	}
}

// TestWriteHTML_EvidenceAppendixShowsKeyEvidence guards RPT-003: the
// Evidence Appendix previously only showed identity + fingerprint, so an
// operator opening it looking for the actual facts (kubelet version,
// disruptionsAllowed, etc.) found nothing — the concrete Evidence lines
// must now be visible in the same table.
func TestWriteHTML_EvidenceAppendixShowsKeyEvidence(t *testing.T) {
	rpt := sampleReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "<th>Key evidence</th>") {
		t.Error("Evidence Appendix table missing a Key evidence column header")
	}
	if !strings.Contains(out, "webhook index: 0") || !strings.Contains(out, "ready endpoint address count: 0") {
		t.Error("Evidence Appendix is missing the WH-002 finding's actual evidence lines")
	}
}

// TestWriteHTML_TopRiskCardHasActionRail guards the interactive-summary
// feature: each Top Risk card must render an action rail with a "Next
// step" restatement of its remediation, and — when the finding has a
// SafeFix inspect command — the labeled inspect block plus a copy button.
// A finding with no SafeFix command (API-001 here) must still get the
// rail's navigation buttons, just without the inspect block.
func TestWriteHTML_TopRiskCardHasActionRail(t *testing.T) {
	rpt := globalBlockerReport() // WH-002 (SafeFix), API-001 (no SafeFix), PDB-001 (SafeFix)
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, `class="risk-card-rail"`) {
		t.Fatal("Top Risk card missing the action rail")
	}
	if !strings.Contains(out, "<h4>Next step</h4>") {
		t.Error("action rail missing the \"Next step\" heading")
	}
	if !strings.Contains(out, "kubectl get svc guard-svc -n guard-ns") {
		t.Error("WH-002's card is missing its own inspect command in the rail")
	}
	if !strings.Contains(out, "Inspect current state first. This does not change the cluster.") {
		t.Error("rail missing the required inspect-first helper text")
	}
	if !strings.Contains(out, ">Copy inspect command<") {
		t.Error("rail missing the \"Copy inspect command\" button")
	}

	// API-001 has no RemediationDetail/SafeFix at all — its card must
	// still render (with Next step + nav buttons), just with no inspect
	// block or copy button for it.
	apiCardStart := strings.Index(out, `data-rule-ids="API-001"`)
	if apiCardStart < 0 {
		t.Fatal("API-001 finding row not found")
	}
}

// TestWriteHTML_TopRiskCardButtonsLinkToFingerprintAnchors guards the
// cross-tab navigation: each card's "View full finding"/"View evidence"
// buttons must target this exact finding's row — identified by
// fingerprint, the one stable per-finding identifier — not some other
// finding's row and not a positional/rank-based id that could drift.
func TestWriteHTML_TopRiskCardButtonsLinkToFingerprintAnchors(t *testing.T) {
	rpt := globalBlockerReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		`data-goto-finding="finding-fp-wh002"`,
		`data-goto-evidence="evidence-fp-wh002"`,
		`data-goto-finding="finding-fp-api001"`,
		`data-goto-evidence="evidence-fp-api001"`,
		`data-goto-finding="finding-fp-pdb001"`,
		`data-goto-evidence="evidence-fp-pdb001"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}
}

// TestWriteHTML_FindingAndEvidenceRowsHaveFingerprintAnchors guards the
// other half of the navigation: the actual target rows in the Findings
// tab and Evidence Appendix must carry the matching id, or the rail's
// buttons have nothing to jump to.
func TestWriteHTML_FindingAndEvidenceRowsHaveFingerprintAnchors(t *testing.T) {
	rpt := globalBlockerReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		`id="finding-fp-wh002"`,
		`id="finding-fp-pdb001"`,
		`id="finding-fp-api001"`,
		`id="evidence-fp-wh002"`,
		`id="evidence-fp-pdb001"`,
		`id="evidence-fp-api001"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}
}

// TestWriteHTML_TopRiskInspectCommandCopyIsolatedPerCard guards against a
// copy-target id collision: WH-002 and PDB-001 both have SafeFix
// commands, and their rails' <pre>/button pairs must be scoped by rank so
// each card's copy button copies only its own command, never another
// card's.
func TestWriteHTML_TopRiskInspectCommandCopyIsolatedPerCard(t *testing.T) {
	rpt := globalBlockerReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	// Rank order: WH-002 (global blocker) = 1, API-001 = 2, PDB-001 = 3.
	if !strings.Contains(out, `id="top-risk-1-inspect">kubectl get svc guard-svc -n guard-ns`) {
		t.Error("WH-002's inspect <pre> missing its rank-scoped id or has the wrong command")
	}
	if !strings.Contains(out, `id="top-risk-3-inspect">kubectl scale deployment payment-api -n prod-like --replicas=2`) {
		t.Error("PDB-001's inspect <pre> missing its rank-scoped id or has the wrong command")
	}
	if !strings.Contains(out, `data-copy-target="top-risk-1-inspect"`) || !strings.Contains(out, `data-copy-target="top-risk-3-inspect"`) {
		t.Error("copy buttons missing rank-scoped data-copy-target")
	}
}

// TestWriteHTML_UpgradeGateChecklistShownOnlyWithBlockers guards the
// explicit scope of the new "Upgrade gate" checklist in Start Here: it
// only makes sense once there's at least one blocker (a warning-only
// report has nothing an upgrade gate should hold open for).
func TestWriteHTML_UpgradeGateChecklistShownOnlyWithBlockers(t *testing.T) {
	rpt := globalBlockerReport() // has blockers
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Upgrade gate checklist") {
		t.Error("Start Here missing the Upgrade gate checklist for a report with blockers")
	}
	if !strings.Contains(out, "Blockers must be 0") || !strings.Contains(out, "Change window approved") {
		t.Error("Upgrade gate checklist missing expected items")
	}

	warningOnly := findings.NewReport("1.34", "prod-cluster", "eks", time.Now(), []findings.Finding{
		{
			RuleID: "WH-001", Severity: findings.SeverityWarning, Confidence: findings.TierStaticCertain,
			Message:     "catch-all webhook scope",
			Resources:   []findings.ResourceReference{findings.LiveResource("ValidatingWebhookConfiguration", findings.ScopeCluster, "", "payments-guard", "uid-1")},
			Remediation: "Narrow the webhook's scope.",
			Fingerprint: "fp-wh001-only",
		},
	})
	buf.Reset()
	if err := WriteHTML(warningOnly, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out = buf.String()
	if strings.Contains(out, "Upgrade gate checklist") {
		t.Error("Upgrade gate checklist must not show for a warning-only report with zero blockers")
	}
	if !strings.Contains(out, `class="start-here-fixes"`) {
		t.Error("Start Here's fix-order column must still render for a warning-only report")
	}
}

// TestFirstSentence_StopsAtSentenceBoundaryNotWholeParagraph guards
// against the action rail's "Next step" becoming a verbatim duplicate of
// the card body's "What to do" text — a long, multi-clause remediation
// with no newline (real rules like PDB-001 write exactly this shape) must
// be shortened to a fast-scan restatement, not repeated in full.
func TestFirstSentence_StopsAtSentenceBoundaryNotWholeParagraph(t *testing.T) {
	long := "Safest-first remediation ladder: (1) scale up replicas to create eviction headroom without changing the PDB contract; (2) add topologySpreadConstraints to distribute the disruption cost across nodes; (3) temporarily relax this PDB for the change window, with an explicit revert step in the change ticket. Force-updating the node group to bypass PDBs is a last resort."
	got := firstSentence(long)
	if got == long {
		t.Fatalf("firstSentence returned the entire paragraph verbatim, want a short restatement")
	}
	if len(got) > 190 {
		t.Errorf("firstSentence(%d chars) = %q (%d chars), want it capped short", len(long), got, len(got))
	}

	short := "Narrow the webhook's scope."
	if got := firstSentence(short); got != short {
		t.Errorf("firstSentence(%q) = %q, want it unchanged (already one short sentence)", short, got)
	}

	noPeriod := "Run: aws eks update-addon --cluster-name <cluster> --addon-name vpc-cni"
	if got := firstSentence(noPeriod); got != noPeriod {
		t.Errorf("firstSentence(%q) = %q, want it unchanged (short, no sentence boundary)", noPeriod, got)
	}
}

// TestWriteHTML_ActionRailEscapesPlaceholderSyntax guards the same
// html/template auto-escaping contract as the rest of the report: a
// remediation/command string containing shell placeholder syntax like
// `<cluster>` must render as literal text in the rail, not be interpreted
// as an HTML tag and silently dropped.
func TestWriteHTML_ActionRailEscapesPlaceholderSyntax(t *testing.T) {
	rpt := sampleReport() // WH-002's Remediation contains "<cluster>"
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "&lt;cluster&gt;") {
		t.Error("action rail's Next step text did not escape \"<cluster>\" placeholder syntax")
	}
	if strings.Contains(out, "<cluster>") {
		t.Error("action rail rendered raw, unescaped \"<cluster>\" — would be silently dropped as an unknown tag by browsers")
	}
}

// TestWriteHTML_EKSClusterAbsent guards the "no upgrade blocker" contract
// (findings.EKSClusterInfo's doc comment): a scan with no EKS metadata at
// all (cluster-only, or AWS enrichment unavailable) must render none of the
// EKS banner chips, not empty ones.
func TestWriteHTML_EKSClusterAbsent(t *testing.T) {
	rpt := sampleReport() // EKSCluster is nil
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, label := range []string{"<dt>Region</dt>", "<dt>EKS version</dt>", "<dt>Platform version</dt>", "<dt>EKS status</dt>", "<dt>Support</dt>", "<dt>Endpoint access</dt>"} {
		if strings.Contains(out, label) {
			t.Errorf("HTML output contains %q with no EKSCluster set — must be hidden entirely, not shown empty", label)
		}
	}
}

// TestWriteHTML_EKSClusterMetadataRenders guards the EKS provider-depth
// banner chips: present and correctly labeled when EKSCluster is set, with
// only the fields AWS actually reported (a cluster predating extended
// support has no SupportType, and that chip must not render at all).
func TestWriteHTML_EKSClusterMetadataRenders(t *testing.T) {
	rpt := sampleReport()
	rpt.EKSCluster = &findings.EKSClusterInfo{
		ClusterName:     "prod-cluster",
		Region:          "ap-south-1",
		Version:         "1.29",
		PlatformVersion: "eks.5",
		Status:          "ACTIVE",
		SupportType:     "EXTENDED",
		EndpointAccess:  "public",
	}
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"<dt>Region</dt><dd>ap-south-1</dd>",
		"<dt>EKS version</dt><dd>1.29</dd>",
		"<dt>Platform version</dt><dd>eks.5</dd>",
		"<dt>EKS status</dt><dd>ACTIVE</dd>",
		"<dt>Support</dt><dd>Extended support</dd>",
		"<dt>Endpoint access</dt><dd>Public</dd>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing EKS metadata chip %q", want)
		}
	}
}

// TestWriteHTML_EKSClusterPartialMetadataOmitsEmptyChips guards a partial
// DescribeCluster response (e.g. an older cluster with no UpgradePolicy) —
// only the fields that are actually present render as chips.
func TestWriteHTML_EKSClusterPartialMetadataOmitsEmptyChips(t *testing.T) {
	rpt := sampleReport()
	rpt.EKSCluster = &findings.EKSClusterInfo{ClusterName: "prod-cluster", Version: "1.29", Status: "ACTIVE"}
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "<dt>EKS status</dt><dd>ACTIVE</dd>") {
		t.Error("HTML output missing EKS status chip")
	}
	for _, label := range []string{"<dt>Region</dt>", "<dt>Platform version</dt>", "<dt>Support</dt>", "<dt>Endpoint access</dt>"} {
		if strings.Contains(out, label) {
			t.Errorf("HTML output contains %q for a field AWS didn't report — must stay hidden", label)
		}
	}
}

// TestWriteHTML_EKSAddonsAbsent guards a scan with no EKS add-on inventory
// (cluster-only, or an EKS cluster with zero installed managed add-ons) —
// the whole section must be hidden, not shown empty.
func TestWriteHTML_EKSAddonsAbsent(t *testing.T) {
	rpt := sampleReport() // EKSAddons is nil
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	if strings.Contains(buf.String(), "EKS add-ons") {
		t.Error("HTML output contains the EKS add-ons section with no EKSAddons set — must be hidden entirely")
	}
}

// TestWriteHTML_EKSAddonsInventoryRendersAllThreeStates guards the full
// add-on inventory table — every installed add-on shown (not just
// incompatible ones), each with the correct status pill.
func TestWriteHTML_EKSAddonsInventoryRendersAllThreeStates(t *testing.T) {
	rpt := sampleReport()
	rpt.EKSAddons = []findings.EKSAddonInfo{
		{Name: "vpc-cni", CurrentVersion: "v1.18.1-eksbuild.1", CompatibleVersions: []string{"v1.18.1-eksbuild.1", "v1.18.2-eksbuild.1"}, Compatible: true},
		{Name: "coredns", CurrentVersion: "v1.10.1-eksbuild.1", CompatibleVersions: []string{"v1.11.0-eksbuild.1"}, Compatible: false},
		{Name: "kube-proxy", CurrentVersion: "v1.29.0-eksbuild.1", VerificationUnavailable: true},
	}
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "EKS add-ons") {
		t.Fatal("HTML output missing the EKS add-ons section")
	}
	if !strings.Contains(out, "EKS does not automatically update add-ons after a Kubernetes minor version upgrade") {
		t.Error("HTML output missing the explicit no-auto-update safety wording")
	}
	for _, want := range []string{
		"<td>vpc-cni</td>", "<td>v1.18.1-eksbuild.1</td>", `<span class="badge-clean">Compatible</span>`,
		"<td>coredns</td>", "<td>v1.10.1-eksbuild.1</td>", `<span class="badge-blocked">Needs update</span>`,
		"<td>kube-proxy</td>", "<td>v1.29.0-eksbuild.1</td>", `<span class="badge-warn">Verification unavailable</span>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q in the EKS add-ons table", want)
		}
	}
}

func TestWriteHTML_EKSNodegroupsInventoryRendersReadinessTable(t *testing.T) {
	rpt := sampleReport()
	rpt.Provider = "eks"
	rpt.EKSCluster = &findings.EKSClusterInfo{ClusterName: "prod", Status: "ACTIVE"}
	desired, min, max := int32(3), int32(3), int32(8)
	maxUnavailable := int32(1)
	rpt.EKSNodegroups = []findings.EKSNodegroupInfo{{
		Name:              "ng-app",
		Status:            "ACTIVE",
		Version:           "1.32",
		ReleaseVersion:    "1.32.7-20260601",
		AMIType:           "AL2023_x86_64_STANDARD",
		CapacityType:      "ON_DEMAND",
		DesiredSize:       &desired,
		MinSize:           &min,
		MaxSize:           &max,
		MaxUnavailable:    &maxUnavailable,
		HealthIssues:      []findings.EKSNodegroupHealthIssue{{Code: "AccessDenied", Message: "node role cannot call API"}},
		ReadinessStatus:   "Review required",
		AutoScalingGroups: []string{"eks-ng-app-asg"},
	}}
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"EKS managed node groups",
		"Self-managed nodes are not listed by that API",
		"<td>ng-app</td>",
		"<td>ACTIVE</td>",
		"<td>1.32</td>",
		"<td>1.32.7-20260601</td>",
		"<td>AL2023_x86_64_STANDARD</td>",
		"<td>ON_DEMAND</td>",
		"<td>3 / 3 / 8</td>",
		"<td>maxUnavailable: 1</td>",
		"<td>AccessDenied</td>",
		`<span class="badge-warn">Review required</span>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q in EKS node groups table", want)
		}
	}
}

func TestWriteHTML_EKSNodegroupsEmptyStateMentionsSelfManagedScope(t *testing.T) {
	rpt := sampleReport()
	rpt.Provider = "eks"
	rpt.EKSCluster = &findings.EKSClusterInfo{ClusterName: "prod", Status: "ACTIVE"}
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "No EKS managed node groups found. Self-managed nodes are not listed by the EKS ListNodegroups API.") {
		t.Error("HTML output missing EKS managed node group empty-state scope wording")
	}
}

func TestWriteHTML_EKSUpgradeInsightsInventoryRendersTable(t *testing.T) {
	rpt := sampleReport()
	rpt.Provider = "eks"
	rpt.EKSCluster = &findings.EKSClusterInfo{ClusterName: "prod", Status: "ACTIVE"}
	rpt.EKSUpgradeInsights = []findings.EKSUpgradeInsightInfo{{
		ID:                 "insight-1",
		Name:               "Deprecated API usage",
		Category:           "UPGRADE_READINESS",
		Status:             "PASSING",
		KubernetesVersion:  "1.34",
		LastRefreshTime:    "2026-06-01T00:00:00Z",
		LastTransitionTime: "2026-06-02T00:00:00Z",
		Recommendation:     "No action required.",
		DeprecationDetails: []string{"usage: policy/v1beta1/podsecuritypolicies"},
	}}
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"EKS Upgrade Insights",
		"AWS-native upgrade readiness checks from Amazon EKS. Insights may be up to 24 hours old; re-check after remediation.",
		"<td>Deprecated API usage</td>",
		`<span class="badge-clean">PASSING</span>`,
		"<td>1.34</td>",
		"<td>2026-06-01T00:00:00Z / 2026-06-02T00:00:00Z</td>",
		"<td>No action required.</td>",
		"<td>usage: policy/v1beta1/podsecuritypolicies</td>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q in EKS Upgrade Insights table", want)
		}
	}
}

func TestWriteHTML_EKSUpgradeInsightsEmptyAndUnavailableStates(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		rpt := sampleReport()
		rpt.Provider = "eks"
		rpt.EKSCluster = &findings.EKSClusterInfo{ClusterName: "prod", Status: "ACTIVE"}
		var buf bytes.Buffer
		if err := WriteHTML(rpt, &buf); err != nil {
			t.Fatalf("WriteHTML: %v", err)
		}
		if !strings.Contains(buf.String(), "No EKS upgrade insights returned.") {
			t.Error("HTML output missing EKS Upgrade Insights empty-state wording")
		}
	})

	t.Run("unavailable", func(t *testing.T) {
		rpt := sampleReport()
		rpt.Provider = "eks"
		rpt.EKSCluster = &findings.EKSClusterInfo{ClusterName: "prod", Status: "ACTIVE"}
		rpt.Coverage.AWS = findings.PlaneCoverage{Status: findings.CoveragePartial, Errors: []string{"list-insights: access denied"}}
		var buf bytes.Buffer
		if err := WriteHTML(rpt, &buf); err != nil {
			t.Fatalf("WriteHTML: %v", err)
		}
		if !strings.Contains(buf.String(), "EKS Upgrade Insights unavailable. Kubernetes findings are still valid.") {
			t.Error("HTML output missing EKS Upgrade Insights unavailable-state wording")
		}
	})
}

// --- Upgrade risk prioritization (Priority/PriorityReason/AffectedScope/
// CanUpgradeContinue) display tests. Sort-order guarantees are tested in
// view_test.go (TestFilterAndSort_PriorityOutranksRuleIDWithinSameSeverity
// and siblings); these tests only guard that each renderer actually shows
// the priority it computed.

func TestWriteTerminal_ShowsPriorityAndReason(t *testing.T) {
	rpt := globalBlockerReport()
	var buf bytes.Buffer
	if err := WriteTerminal(rpt, &buf); err != nil {
		t.Fatalf("WriteTerminal: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"[P1/WH-002]",
		"Priority P1 (do not attempt other remediation until this is fixed): May block kubectl apply/patch/scale, Helm upgrades, and controller reconciliation.",
		"[P2/API-001]",
		"[P3/PDB-001]",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("terminal output missing %q\n--- full output ---\n%s", want, out)
		}
	}
}

func TestWriteMarkdown_ShowsPriorityAndCanUpgradeContinue(t *testing.T) {
	rpt := globalBlockerReport()
	var buf bytes.Buffer
	if err := WriteMarkdown(rpt, &buf); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"### `P1` `WH-002`",
		"Can upgrade continue: No",
		"> **Why this matters (P1):** May block kubectl apply/patch/scale, Helm upgrades, and controller reconciliation.",
		"### `P2` `API-001`",
		"### `P3` `PDB-001`",
		"| Priority | Rule ID | Severity | Confidence | Resource | Fingerprint |",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown output missing %q\n--- full output ---\n%s", want, out)
		}
	}
	if got := strings.Count(out, "Can upgrade continue: No"); got != 3 {
		t.Errorf("markdown output has %d no-continue blocker findings, want 3\n--- full output ---\n%s", got, out)
	}
	if strings.Contains(out, "Can upgrade continue: Yes") {
		t.Errorf("markdown output still says a blocker can continue\n--- full output ---\n%s", out)
	}
}

func TestWriteHTML_ShowsPriorityPillsAndDetailBlock(t *testing.T) {
	rpt := globalBlockerReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		`<span class="priority-pill p1" title="May block kubectl apply/patch/scale, Helm upgrades, and controller reconciliation.">P1</span>`,
		`<div class="priority-detail p1">`,
		"<strong>Priority P1</strong>",
		"Can upgrade continue: No",
		"Affected scope: global",
		`<span class="priority-pill p2"`,
		`<span class="priority-pill p3"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing %q", want)
		}
	}
}

func TestWriteHTML_EvidenceAppendixShowsPriorityColumn(t *testing.T) {
	rpt := globalBlockerReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "<th>Priority</th><th>Rule ID</th>") {
		t.Error("Evidence Appendix header missing Priority column")
	}
}

// TestWriteHTML_TopRisksShowsPriorityLegend guards the standalone legend
// line next to Top Risks — someone encountering a P1/P4 pill for the
// first time shouldn't need to hover a tooltip to learn what the scale
// means at all.
func TestWriteHTML_TopRisksShowsPriorityLegend(t *testing.T) {
	rpt := globalBlockerReport()
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "Priority ranks upgrade urgency: P1 = fix now, P2 = fix before upgrade, P3 = fix before drain/maintenance, P4 = stabilize before starting.") {
		t.Error("HTML output missing the P1-P4 priority legend near Top Risks")
	}
}

// TestWriteHTML_NoTopRisksNoLegend guards against the legend rendering
// with nothing to explain (a clean report has no Top Risks section at
// all).
func TestWriteHTML_NoTopRisksNoLegend(t *testing.T) {
	rpt := findings.NewReport("1.36", "clean-cluster", "", time.Now(), nil)
	var buf bytes.Buffer
	if err := WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	if strings.Contains(buf.String(), "priority-legend") {
		t.Error("HTML output contains the priority legend with no Top Risks — must be hidden entirely")
	}
}
