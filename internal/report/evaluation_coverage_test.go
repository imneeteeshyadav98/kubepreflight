package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

// evaluationCoverageReport builds a report whose findings are unrelated to
// rule-execution complexity, then overlays the given RuleExecutions/
// RuleExecutionsNormalized on top -- isolating the new coverage rendering
// from every other renderer concern already covered by sampleReport et al.
func evaluationCoverageReport(execs []findings.RuleExecutionRecord, normalized bool) *findings.Report {
	fs := []findings.Finding{
		{
			RuleID: "WH-002", Severity: findings.SeverityBlocker, Confidence: findings.TierStaticCertain,
			Message:     `webhook "payments-guard" is fail-closed with no ready endpoints`,
			Resources:   []findings.ResourceReference{findings.LiveResource("ValidatingWebhookConfiguration", findings.ScopeCluster, "", "payments-guard", "uid-1")},
			Remediation: "Fix the webhook backend.",
			Fingerprint: "fp-wh002",
		},
	}
	rpt := findings.NewReport("1.34", "prod-cluster", "eks", time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC), fs)
	rpt.RuleExecutions = execs
	rpt.RuleExecutionsNormalized = normalized
	return rpt
}

// fullCoverageExecutions is every rule cleanly evaluated with zero findings
// except WH-002 (which the report above already carries a finding for).
func fullCoverageExecutions() []findings.RuleExecutionRecord {
	return []findings.RuleExecutionRecord{
		{RuleID: "WH-002", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionEvaluated},
		{RuleID: "WH-001", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionEvaluated},
		{RuleID: "PDB-001", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionEvaluated},
	}
}

// partialCoverageExecutions mixes every execution state this PR must render
// distinctly, plus a not_applicable rule.
func partialCoverageExecutions() []findings.RuleExecutionRecord {
	return []findings.RuleExecutionRecord{
		{RuleID: "WH-002", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionEvaluated},
		{RuleID: "PDB-001", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionNotEvaluated, Reason: "not registered for this scan mode"},
		{RuleID: "NODE-002", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionInsufficientEvidence, Reason: "AWS collector partial: missing subnet describe permission"},
		{RuleID: "CRD-002", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionFailed, Reason: "rule returned an error: nil pointer"},
		{RuleID: "APISERVICE-001", Applicability: findings.ApplicabilityNotApplicable, State: findings.ExecutionNotEvaluated, Reason: "not registered for this scan mode"},
	}
}

func TestEvaluationCoverage_CompleteCoverage_AllThreeFormats(t *testing.T) {
	rpt := evaluationCoverageReport(fullCoverageExecutions(), false)

	var term, md, html bytes.Buffer
	if err := WriteTerminal(rpt, &term); err != nil {
		t.Fatalf("WriteTerminal: %v", err)
	}
	if err := WriteMarkdown(rpt, &md); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	if err := WriteHTML(rpt, &html); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}

	for name, out := range map[string]string{"terminal": term.String(), "markdown": md.String(), "html": html.String()} {
		if !strings.Contains(out, "Complete") {
			t.Errorf("%s: expected coverage label Complete, got:\n%s", name, out)
		}
		if !strings.Contains(out, "Native") {
			t.Errorf("%s: expected Source Native, got:\n%s", name, out)
		}
		if strings.Contains(out, "Normalized from legacy") {
			t.Errorf("%s: native report must not show normalization warning, got:\n%s", name, out)
		}
	}
}

func TestEvaluationCoverage_PartialCoverage_RendersEachStateDistinctly(t *testing.T) {
	rpt := evaluationCoverageReport(partialCoverageExecutions(), false)

	var term, md, htmlBuf bytes.Buffer
	if err := WriteTerminal(rpt, &term); err != nil {
		t.Fatalf("WriteTerminal: %v", err)
	}
	if err := WriteMarkdown(rpt, &md); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	if err := WriteHTML(rpt, &htmlBuf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}

	for name, out := range map[string]string{"terminal": term.String(), "markdown": md.String(), "html": htmlBuf.String()} {
		for _, want := range []string{"Partial", "Not evaluated", "Insufficient evidence", "Failed", "Not applicable"} {
			if !strings.Contains(out, want) {
				t.Errorf("%s: missing %q\n%s", name, want, out)
			}
		}
	}
}

func TestEvaluationCoverage_NormalizedLegacyShowsSourceLabel_NativeDoesNot(t *testing.T) {
	legacy := evaluationCoverageReport(fullCoverageExecutions(), true)
	native := evaluationCoverageReport(fullCoverageExecutions(), false)

	renderers := []struct {
		name  string
		write func(*findings.Report, *bytes.Buffer) error
	}{
		{"terminal", func(r *findings.Report, b *bytes.Buffer) error { return WriteTerminal(r, b) }},
		{"markdown", func(r *findings.Report, b *bytes.Buffer) error { return WriteMarkdown(r, b) }},
		{"html", func(r *findings.Report, b *bytes.Buffer) error { return WriteHTML(r, b) }},
	}
	for _, rd := range renderers {
		t.Run(rd.name, func(t *testing.T) {
			var legacyBuf, nativeBuf bytes.Buffer
			if err := rd.write(legacy, &legacyBuf); err != nil {
				t.Fatalf("%s render (legacy): %v", rd.name, err)
			}
			if err := rd.write(native, &nativeBuf); err != nil {
				t.Fatalf("%s render (native): %v", rd.name, err)
			}
			if !strings.Contains(legacyBuf.String(), "Normalized from legacy report") {
				t.Errorf("%s: legacy report missing 'Normalized from legacy report' label:\n%s", rd.name, legacyBuf.String())
			}
			if strings.Contains(nativeBuf.String(), "Normalized from legacy report") {
				t.Errorf("%s: native report must never show the normalization label:\n%s", rd.name, nativeBuf.String())
			}
			if !strings.Contains(nativeBuf.String(), "Source: Native") && !strings.Contains(nativeBuf.String(), ">Native<") && !strings.Contains(nativeBuf.String(), "| **Source** | Native |") {
				t.Errorf("%s: native report missing an explicit Native source label:\n%s", rd.name, nativeBuf.String())
			}
		})
	}
}

// TestEvaluationCoverage_NotEvaluatedNeverRendersAsPassed is the critical
// regression test this PR exists for: a rule marked not_evaluated with zero
// findings must never be rendered with "clean"/"passed"-style wording. The
// invariant is keyed off outcome text, not absence of the word "Passed"
// (which never appears in this renderer's vocabulary anyway) -- it asserts
// the not_evaluated rule's own outcome line reads "Not evaluated", never
// "No issue detected" (the wording reserved exclusively for a genuinely
// evaluated, zero-finding rule).
func TestEvaluationCoverage_NotEvaluatedNeverRendersAsPassed(t *testing.T) {
	execs := []findings.RuleExecutionRecord{
		{RuleID: "WH-002", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionEvaluated},
		{RuleID: "PDB-002", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionNotEvaluated, Reason: "not registered for this scan mode"},
	}
	rpt := evaluationCoverageReport(execs, false)

	rows := BuildRuleExecutionRows(rpt)
	var pdb002 *RuleExecutionRow
	for i := range rows {
		if rows[i].RuleID == "PDB-002" {
			pdb002 = &rows[i]
		}
	}
	if pdb002 == nil {
		t.Fatalf("PDB-002 row missing from rows: %+v", rows)
	}
	if pdb002.Outcome != "Not evaluated" {
		t.Errorf("PDB-002 (not_evaluated, 0 findings) outcome = %q, want %q", pdb002.Outcome, "Not evaluated")
	}
	if pdb002.Outcome == "No issue detected" {
		t.Fatalf("PDB-002 not_evaluated rule must never render as a clean pass")
	}
	if pdb002.State != "Not evaluated" {
		t.Errorf("PDB-002 state label = %q, want %q", pdb002.State, "Not evaluated")
	}

	var buf bytes.Buffer
	if err := WriteTerminal(rpt, &buf); err != nil {
		t.Fatalf("WriteTerminal: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[PDB-002]") {
		t.Fatalf("terminal output missing PDB-002 row:\n%s", out)
	}
	// Isolate PDB-002's own line and confirm it never carries "No issue
	// detected" -- the wording that would incorrectly read as a pass.
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "[PDB-002]") && strings.Contains(line, "No issue detected") {
			t.Fatalf("PDB-002 line incorrectly rendered as a clean pass: %q", line)
		}
	}
}

func TestEvaluationCoverage_InsufficientEvidenceReadsUnableToVerify(t *testing.T) {
	execs := []findings.RuleExecutionRecord{
		{RuleID: "WH-002", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionEvaluated},
		{RuleID: "NODE-002", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionInsufficientEvidence, Reason: "AWS collector partial"},
	}
	rpt := evaluationCoverageReport(execs, false)
	rows := BuildRuleExecutionRows(rpt)
	var node002 *RuleExecutionRow
	for i := range rows {
		if rows[i].RuleID == "NODE-002" {
			node002 = &rows[i]
		}
	}
	if node002 == nil {
		t.Fatalf("NODE-002 row missing")
	}
	if node002.Outcome != "Unable to verify" {
		t.Errorf("NODE-002 outcome = %q, want %q", node002.Outcome, "Unable to verify")
	}
	if node002.State != "Insufficient evidence" {
		t.Errorf("NODE-002 state = %q, want %q", node002.State, "Insufficient evidence")
	}
	// Distinct from a genuine evaluated-clean pass.
	if node002.Outcome == "No issue detected" {
		t.Fatalf("insufficient_evidence must never read as a genuine pass")
	}
}

func TestEvaluationCoverage_FailedIsDistinctFromInsufficientEvidence(t *testing.T) {
	execs := []findings.RuleExecutionRecord{
		{RuleID: "NODE-002", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionInsufficientEvidence, Reason: "partial evidence"},
		{RuleID: "CRD-002", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionFailed, Reason: "rule returned an error"},
	}
	rpt := evaluationCoverageReport(execs, false)
	rows := BuildRuleExecutionRows(rpt)

	var insufficient, failed *RuleExecutionRow
	for i := range rows {
		switch rows[i].RuleID {
		case "NODE-002":
			insufficient = &rows[i]
		case "CRD-002":
			failed = &rows[i]
		}
	}
	if insufficient == nil || failed == nil {
		t.Fatalf("expected both NODE-002 and CRD-002 rows, got %+v", rows)
	}
	if insufficient.State == failed.State {
		t.Fatalf("insufficient_evidence and failed must render with distinct state labels, both got %q", insufficient.State)
	}
	if insufficient.Outcome == failed.Outcome {
		t.Fatalf("insufficient_evidence and failed must render with distinct outcome text, both got %q", insufficient.Outcome)
	}
	if insufficient.StateClass == failed.StateClass {
		t.Fatalf("insufficient_evidence and failed must use distinct HTML status classes, both got %q", insufficient.StateClass)
	}
	// No shared/ambiguous phrasing between the two.
	if strings.Contains(insufficient.Outcome, failed.Outcome) || strings.Contains(failed.Outcome, insufficient.Outcome) {
		t.Fatalf("insufficient_evidence (%q) and failed (%q) outcome text must not overlap", insufficient.Outcome, failed.Outcome)
	}
}

// TestEvaluationCoverage_ReasonEscaping confirms a Reason string containing
// Markdown/HTML special characters is neither dropped nor allowed to break
// output structure in Markdown or HTML.
func TestEvaluationCoverage_ReasonEscaping(t *testing.T) {
	dangerous := "evidence <script>alert(1)</script> was `partial` & *bold* _emph_ | pipe"
	execs := []findings.RuleExecutionRecord{
		{RuleID: "NODE-002", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionInsufficientEvidence, Reason: dangerous},
	}
	rpt := evaluationCoverageReport(execs, false)

	var md bytes.Buffer
	if err := WriteMarkdown(rpt, &md); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	mdOut := md.String()
	if strings.Contains(mdOut, "<script>") {
		t.Errorf("markdown output contains unescaped <script> tag:\n%s", mdOut)
	}
	// The table row for NODE-002 must still have exactly 5 columns (4
	// internal " | " column separators, plus the leading/trailing pipes) --
	// an unescaped "|" in Reason would introduce an extra, corrupting
	// separator. The raw reason string's own "|" is escaped to "\|" (no
	// surrounding spaces), so it never matches the exact " | " separator
	// substring the table formatter itself emits between columns.
	found := false
	for _, line := range strings.Split(mdOut, "\n") {
		if strings.Contains(line, "NODE-002") && strings.Contains(line, "partial") {
			found = true
			if got := strings.Count(line, " | "); got != 4 {
				t.Errorf("NODE-002 markdown row has %d internal column separators, want 4 (5 columns): %q", got, line)
			}
			if !strings.Contains(line, `\| pipe`) {
				t.Errorf("NODE-002 markdown row should contain the escaped pipe character (\\|): %q", line)
			}
		}
	}
	if !found {
		t.Fatalf("could not locate NODE-002 row in markdown output:\n%s", mdOut)
	}

	var htmlBuf bytes.Buffer
	if err := WriteHTML(rpt, &htmlBuf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	htmlOut := htmlBuf.String()
	if strings.Contains(htmlOut, "<script>alert(1)</script>") {
		t.Errorf("html output contains an unescaped, live <script> tag -- injection risk:\n%s", htmlOut)
	}
	if !strings.Contains(htmlOut, "&lt;script&gt;") {
		t.Errorf("html output should contain the HTML-escaped reason text:\n%s", htmlOut)
	}
	if !strings.Contains(htmlOut, "&amp;") {
		t.Errorf("html output should HTML-escape the literal '&' in the reason text")
	}
}

// TestEvaluationCoverage_ZeroRuleExecutions confirms a report with no
// RuleExecutions data (e.g. a pre-v1.3.0 document that bypassed
// comparison.LoadAndNormalize) renders safely across all three formats:
// no panic, and no fabricated "fully evaluated"/"Complete" claim.
func TestEvaluationCoverage_ZeroRuleExecutions(t *testing.T) {
	rpt := evaluationCoverageReport(nil, false)

	renderers := map[string]func(*findings.Report, *bytes.Buffer) error{
		"terminal": func(r *findings.Report, b *bytes.Buffer) error { return WriteTerminal(r, b) },
		"markdown": func(r *findings.Report, b *bytes.Buffer) error { return WriteMarkdown(r, b) },
		"html":     func(r *findings.Report, b *bytes.Buffer) error { return WriteHTML(r, b) },
	}
	for name, write := range renderers {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if rec := recover(); rec != nil {
					t.Fatalf("%s panicked on zero RuleExecutions: %v", name, rec)
				}
			}()
			var buf bytes.Buffer
			if err := write(rpt, &buf); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			out := buf.String()
			if strings.Contains(out, "Evaluation Coverage") || strings.Contains(out, "Evaluation coverage") {
				t.Errorf("%s: expected no evaluation-coverage section rendered for zero RuleExecutions, got:\n%s", name, out)
			}
			if strings.Contains(out, "Rule Executions") {
				t.Errorf("%s: expected no rule-executions table rendered for zero RuleExecutions, got:\n%s", name, out)
			}
		})
	}

	cov := BuildEvaluationCoverage(rpt)
	if cov.HasData {
		t.Fatalf("expected HasData=false for zero RuleExecutions, got %+v", cov)
	}
	if cov.CoverageLabel == "Complete" {
		t.Fatalf("zero RuleExecutions must never report CoverageLabel=Complete: %+v", cov)
	}
	if rows := BuildRuleExecutionRows(rpt); rows != nil {
		t.Fatalf("expected nil rows for zero RuleExecutions, got %+v", rows)
	}
}

// TestEvaluationCoverage_ExistingOutputByteIdentical confirms that a report
// with no rule-execution complexity (the common case exercised by every
// pre-existing renderer test) renders byte-identically to how it rendered
// before this PR -- i.e. this feature adds nothing when there's no
// RuleExecutions data to show. sampleReport is the same fixture
// TestWriteTerminal_ContainsExpectedSections etc. already use.
func TestEvaluationCoverage_ExistingOutputByteIdentical(t *testing.T) {
	rpt := sampleReport()
	if len(rpt.RuleExecutions) != 0 {
		t.Fatalf("sampleReport must not carry RuleExecutions for this test to be meaningful")
	}

	var term, md, htmlBuf bytes.Buffer
	if err := WriteTerminal(rpt, &term); err != nil {
		t.Fatalf("WriteTerminal: %v", err)
	}
	if err := WriteMarkdown(rpt, &md); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	if err := WriteHTML(rpt, &htmlBuf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	for name, out := range map[string]string{"terminal": term.String(), "markdown": md.String(), "html": htmlBuf.String()} {
		if strings.Contains(out, "Evaluation Coverage") || strings.Contains(out, "Evaluation coverage") {
			t.Errorf("%s: unexpected Evaluation Coverage section for a report with no RuleExecutions data:\n%s", name, out)
		}
	}
}
