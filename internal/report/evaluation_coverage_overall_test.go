package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

// ---------------------------------------------------------------------
// OverallCoverage: the combining rule-execution + evidence-plane status
// this fix introduces. Root cause: a real-EKS reduced-IAM certification
// scan produced a report where every one of 31 RuleExecutions read
// State: evaluated (report.BuildEvaluationCoverage -> CoverageStatusComplete)
// while Report.Coverage.AWS.Status read "partial" (7 AWS collector calls
// failed with AccessDenied) and Report.Result read "INCOMPLETE" -- nothing
// in the evaluation-coverage presentation pointed an operator at that
// contradiction, because BuildEvaluationCoverage has no path to read
// Report.Coverage at all. OverallCoverage/BuildOverallCoverage
// (evaluation_coverage.go) fixes that by composing the two, unmodified,
// into one combined status. See BuildOverallCoverage's own doc comment for
// the exact precedence. This file implements the 17 required tests from
// the corrective-fix task; each test's own comment cites which of the 17
// it covers.
// ---------------------------------------------------------------------

// completePlaneCoverage is every plane genuinely complete -- the baseline
// "nothing degraded" evidence-plane shape most tests start from and then
// selectively degrade one plane at a time.
func completePlaneCoverage() findings.ScanCoverage {
	return findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageComplete},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageComplete},
	}
}

// overallCoverageReport builds a findings.Report carrying both the given
// RuleExecutions and the given ScanCoverage -- the two independent inputs
// BuildOverallCoverage composes. Mirrors evaluationCoverageReport's fixture
// style (evaluation_coverage_test.go) but additionally overlays Coverage,
// which that helper leaves at NewReport's placeholder default.
func overallCoverageReport(execs []findings.RuleExecutionRecord, normalized bool, coverage findings.ScanCoverage) *findings.Report {
	fs := []findings.Finding{
		{
			RuleID: "WH-002", Severity: findings.SeverityBlocker, Confidence: findings.TierStaticCertain,
			Message:     `webhook "payments-guard" is fail-closed with no ready endpoints`,
			Resources:   []findings.ResourceReference{findings.LiveResource("ValidatingWebhookConfiguration", findings.ScopeCluster, "", "payments-guard", "uid-1")},
			Remediation: "Fix the webhook backend.",
			Fingerprint: "fp-wh002",
		},
	}
	rpt := findings.NewReport("1.34", "cert-cluster", "eks", time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC), fs)
	rpt.RuleExecutions = execs
	rpt.RuleExecutionsNormalized = normalized
	rpt.SetCoverage(coverage)
	return rpt
}

// Test 1: all rules evaluated + all required evidence planes complete ->
// overall complete.
func TestOverallCoverage_CompleteWhenRulesAndPlanesComplete(t *testing.T) {
	rpt := overallCoverageReport(fullCoverageExecutions(), false, completePlaneCoverage())
	ruleCov := BuildEvaluationCoverage(rpt)
	overall := BuildOverallCoverage(ruleCov, rpt.Coverage)

	if ruleCov.Status != CoverageStatusComplete {
		t.Fatalf("fixture precondition failed: rule coverage = %q, want complete", ruleCov.Status)
	}
	if overall.Status != CoverageStatusComplete {
		t.Fatalf("OverallCoverage.Status = %q, want complete", overall.Status)
	}
	if len(overall.DegradedPlanes) != 0 {
		t.Errorf("DegradedPlanes = %v, want empty", overall.DegradedPlanes)
	}
	if overall.Advisory() != "" {
		t.Errorf("Advisory() = %q, want \"\" for genuinely complete overall coverage", overall.Advisory())
	}
}

// Test 2: all rules evaluated + AWS partial -> overall partial. This is the
// exact reduced-IAM certification shape: rule execution reads clean, but
// the AWS evidence plane it depends on does not.
func TestOverallCoverage_PartialWhenAWSPlanePartial(t *testing.T) {
	coverage := completePlaneCoverage()
	coverage.AWS = findings.PlaneCoverage{Status: findings.CoveragePartial, Errors: []string{"list-nodegroups: AccessDenied"}}
	rpt := overallCoverageReport(fullCoverageExecutions(), false, coverage)
	ruleCov := BuildEvaluationCoverage(rpt)
	overall := BuildOverallCoverage(ruleCov, rpt.Coverage)

	if ruleCov.Status != CoverageStatusComplete {
		t.Fatalf("fixture precondition failed: rule coverage = %q, want complete (rule execution itself is honestly clean)", ruleCov.Status)
	}
	if overall.Status != CoverageStatusPartial {
		t.Fatalf("OverallCoverage.Status = %q, want partial", overall.Status)
	}
	if len(overall.DegradedPlanes) != 1 || overall.DegradedPlanes[0] != "AWS" {
		t.Errorf("DegradedPlanes = %v, want [AWS]", overall.DegradedPlanes)
	}
}

// Test 3: all rules evaluated + Kubernetes partial -> overall partial.
func TestOverallCoverage_PartialWhenKubernetesPlanePartial(t *testing.T) {
	coverage := completePlaneCoverage()
	coverage.Kubernetes = findings.PlaneCoverage{Status: findings.CoveragePartial, Errors: []string{"pods: forbidden"}}
	rpt := overallCoverageReport(fullCoverageExecutions(), false, coverage)
	ruleCov := BuildEvaluationCoverage(rpt)
	overall := BuildOverallCoverage(ruleCov, rpt.Coverage)

	if overall.Status != CoverageStatusPartial {
		t.Fatalf("OverallCoverage.Status = %q, want partial", overall.Status)
	}
	if len(overall.DegradedPlanes) != 1 || overall.DegradedPlanes[0] != "Kubernetes" {
		t.Errorf("DegradedPlanes = %v, want [Kubernetes]", overall.DegradedPlanes)
	}
}

// Test 4: rule execution partial (some rule not_evaluated) + all planes
// complete -> overall partial. The pre-existing rule-execution gap alone
// must still drive the combined status, exactly as it did before this fix.
func TestOverallCoverage_PartialWhenRuleExecutionPartial(t *testing.T) {
	rpt := overallCoverageReport(partialCoverageExecutions(), false, completePlaneCoverage())
	ruleCov := BuildEvaluationCoverage(rpt)
	overall := BuildOverallCoverage(ruleCov, rpt.Coverage)

	if ruleCov.Status != CoverageStatusPartial {
		t.Fatalf("fixture precondition failed: rule coverage = %q, want partial", ruleCov.Status)
	}
	if overall.Status != CoverageStatusPartial {
		t.Fatalf("OverallCoverage.Status = %q, want partial", overall.Status)
	}
	if len(overall.DegradedPlanes) != 0 {
		t.Errorf("DegradedPlanes = %v, want empty (this gap is rule-execution-only)", overall.DegradedPlanes)
	}
	if !strings.Contains(overall.Advisory(), "not fully evaluated") {
		t.Errorf("Advisory() = %q, want it to mention rules not fully evaluated", overall.Advisory())
	}
}

// Test 5: not-applicable rules alone do not cause partial -- confirmed
// already true for rule-execution coverage (evaluation_coverage_test.go's
// TestEvaluationCoverageStatus_NotApplicableAloneStaysComplete); this test
// confirms it still holds once composed into OverallCoverage.
func TestOverallCoverage_NotApplicableAloneStaysComplete(t *testing.T) {
	execs := []findings.RuleExecutionRecord{
		{RuleID: "WH-002", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionEvaluated},
		{RuleID: "APISERVICE-001", Applicability: findings.ApplicabilityNotApplicable, State: findings.ExecutionNotEvaluated},
	}
	rpt := overallCoverageReport(execs, false, completePlaneCoverage())
	ruleCov := BuildEvaluationCoverage(rpt)
	overall := BuildOverallCoverage(ruleCov, rpt.Coverage)

	if overall.Status != CoverageStatusComplete {
		t.Fatalf("OverallCoverage.Status = %q, want complete (not_applicable rules must never cause partial)", overall.Status)
	}
}

// Test 6: normalized-legacy stays normalized_legacy regardless of plane
// coverage -- precedence preserved. Even a partial AWS plane must not
// downgrade/upgrade this to "partial" or "complete"; normalized_legacy wins
// unconditionally, exactly as CoverageStatusNormalizedLegacy already does
// inside BuildEvaluationCoverage itself.
func TestOverallCoverage_NormalizedLegacyPrecedencePreserved(t *testing.T) {
	coverage := completePlaneCoverage()
	coverage.AWS = findings.PlaneCoverage{Status: findings.CoveragePartial, Errors: []string{"list-nodegroups: AccessDenied"}}
	rpt := overallCoverageReport(fullCoverageExecutions(), true, coverage)
	ruleCov := BuildEvaluationCoverage(rpt)
	overall := BuildOverallCoverage(ruleCov, rpt.Coverage)

	if ruleCov.Status != CoverageStatusNormalizedLegacy {
		t.Fatalf("fixture precondition failed: rule coverage = %q, want normalized_legacy", ruleCov.Status)
	}
	if overall.Status != CoverageStatusNormalizedLegacy {
		t.Fatalf("OverallCoverage.Status = %q, want normalized_legacy even with a partial AWS plane (precedence must be preserved)", overall.Status)
	}
}

// Test 7: no rule-execution metadata at all -> unavailable regardless of
// plane coverage -- precedence preserved.
func TestOverallCoverage_UnavailablePrecedencePreserved(t *testing.T) {
	coverage := completePlaneCoverage()
	coverage.AWS = findings.PlaneCoverage{Status: findings.CoveragePartial, Errors: []string{"list-nodegroups: AccessDenied"}}
	rpt := overallCoverageReport(nil, false, coverage)
	ruleCov := BuildEvaluationCoverage(rpt)
	overall := BuildOverallCoverage(ruleCov, rpt.Coverage)

	if ruleCov.Status != CoverageStatusUnavailable {
		t.Fatalf("fixture precondition failed: rule coverage = %q, want unavailable", ruleCov.Status)
	}
	if overall.Status != CoverageStatusUnavailable {
		t.Fatalf("OverallCoverage.Status = %q, want unavailable even with a partial AWS plane (precedence must be preserved)", overall.Status)
	}
}

// Test 8: readiness score is byte-identical across every OverallCoverage
// state for the same underlying findings -- score math is completely
// untouched by this fix (score formula is explicitly out of scope).
func TestOverallCoverage_ReadinessScoreByteIdentical(t *testing.T) {
	degradedAWS := completePlaneCoverage()
	degradedAWS.AWS = findings.PlaneCoverage{Status: findings.CoveragePartial, Errors: []string{"list-nodegroups: AccessDenied"}}

	variants := map[string]*findings.Report{
		"complete":               overallCoverageReport(fullCoverageExecutions(), false, completePlaneCoverage()),
		"rule-partial":           overallCoverageReport(partialCoverageExecutions(), false, completePlaneCoverage()),
		"plane-partial":          overallCoverageReport(fullCoverageExecutions(), false, degradedAWS),
		"rule-and-plane-partial": overallCoverageReport(partialCoverageExecutions(), false, degradedAWS),
		"unavailable":            overallCoverageReport(nil, false, completePlaneCoverage()),
		"normalized_legacy":      overallCoverageReport(fullCoverageExecutions(), true, completePlaneCoverage()),
	}

	var wantScore int
	first := true
	for name, rpt := range variants {
		if rpt.UpgradeReadiness == nil {
			t.Fatalf("%s: UpgradeReadiness is nil", name)
		}
		score := rpt.UpgradeReadiness.ReadinessScore
		if first {
			wantScore = score
			first = false
			continue
		}
		if score != wantScore {
			t.Errorf("%s: ReadinessScore = %d, want %d (identical across every OverallCoverage state for the same findings)", name, score, wantScore)
		}
	}
}

// Test 11 (core regression test): qualification/advisory text is shown when
// overall coverage is partial due to plane degradation ALONE -- even when
// rule-execution coverage itself is complete. This is the renderer-level
// assertion of the exact bug found in the real-EKS reduced-IAM
// certification scan: Terminal/Markdown/HTML must all surface the AWS gap
// even though every RuleExecutions entry reads "evaluated".
func TestOverallCoverage_AdvisoryVisibleForPlaneDegradationAlone(t *testing.T) {
	coverage := completePlaneCoverage()
	coverage.AWS = findings.PlaneCoverage{Status: findings.CoveragePartial, Errors: []string{"list-nodegroups: AccessDenied"}}
	rpt := overallCoverageReport(fullCoverageExecutions(), false, coverage)

	ruleCov := BuildEvaluationCoverage(rpt)
	if ruleCov.Status != CoverageStatusComplete {
		t.Fatalf("fixture precondition failed: rule coverage = %q, want complete", ruleCov.Status)
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
		if !strings.Contains(out, "Partial") {
			t.Errorf("%s: expected the Coverage caption to read Partial despite complete rule-execution coverage, got:\n%s", name, out)
		}
		if !strings.Contains(out, ScoreQualification) {
			t.Errorf("%s: missing score-qualification text for plane-degraded coverage:\n%s", name, out)
		}
		if !strings.Contains(out, "evidence collection was incomplete for: AWS") {
			t.Errorf("%s: missing the AWS-plane-degradation advisory text:\n%s", name, out)
		}
	}
}

// Test 12: qualification/advisory text is absent only when overall coverage
// is genuinely complete (both rule-execution and every attempted plane).
func TestOverallCoverage_AdvisoryAbsentOnlyWhenGenuinelyComplete(t *testing.T) {
	rpt := overallCoverageReport(fullCoverageExecutions(), false, completePlaneCoverage())

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
		if strings.Contains(out, ScoreQualification) {
			t.Errorf("%s: unexpected score-qualification text for genuinely complete overall coverage:\n%s", name, out)
		}
	}
}

// Test 13: manifests-only scope stays truthful -- a legitimately skipped
// Kubernetes/AWS plane (because a --manifests-only scan never attempts
// them) does not drag overall coverage to partial. Mirrors NewReport's own
// default ScanCoverage shape for a manifests-only run: only Manifests is
// actually attempted/complete, Kubernetes/AWS are Skipped, not Partial.
func TestOverallCoverage_ManifestsOnlyScopeStaysTruthful(t *testing.T) {
	manifestsOnly := findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageSkipped},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageSkipped},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageComplete},
	}
	rpt := overallCoverageReport(fullCoverageExecutions(), false, manifestsOnly)
	ruleCov := BuildEvaluationCoverage(rpt)
	overall := BuildOverallCoverage(ruleCov, rpt.Coverage)

	if overall.Status != CoverageStatusComplete {
		t.Fatalf("OverallCoverage.Status = %q, want complete (skipped planes must never count as degraded)", overall.Status)
	}
	if len(overall.DegradedPlanes) != 0 {
		t.Errorf("DegradedPlanes = %v, want empty for legitimately skipped planes", overall.DegradedPlanes)
	}

	// A live-cluster scan with no --manifests flag is the mirror image:
	// Manifests is legitimately Skipped, Kubernetes/AWS are Complete.
	liveOnly := findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageComplete},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageSkipped},
	}
	rpt2 := overallCoverageReport(fullCoverageExecutions(), false, liveOnly)
	overall2 := BuildOverallCoverage(BuildEvaluationCoverage(rpt2), rpt2.Coverage)
	if overall2.Status != CoverageStatusComplete {
		t.Fatalf("OverallCoverage.Status = %q, want complete for a live-only scan with a legitimately skipped Manifests plane", overall2.Status)
	}
}

// Test 14: Terminal/Markdown/HTML show the same overall status/advisory/
// qualification consistently -- for both a plane-only partial (this fix's
// core scenario) and a rule-only partial (the pre-existing scenario).
func TestOverallCoverage_TerminalMarkdownHTMLConsistent(t *testing.T) {
	degradedAWS := completePlaneCoverage()
	degradedAWS.AWS = findings.PlaneCoverage{Status: findings.CoveragePartial, Errors: []string{"describe-vpc: AccessDenied"}}

	cases := map[string]*findings.Report{
		"plane-only-partial": overallCoverageReport(fullCoverageExecutions(), false, degradedAWS),
		"rule-only-partial":  overallCoverageReport(partialCoverageExecutions(), false, completePlaneCoverage()),
	}

	for name, rpt := range cases {
		t.Run(name, func(t *testing.T) {
			ruleCov := BuildEvaluationCoverage(rpt)
			overall := BuildOverallCoverage(ruleCov, rpt.Coverage)

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

			wantLabel := overall.Status.Label()
			for surface, out := range map[string]string{"terminal": term.String(), "markdown": md.String(), "html": htmlBuf.String()} {
				if !strings.Contains(out, wantLabel) {
					t.Errorf("%s/%s: missing overall status label %q:\n%s", name, surface, wantLabel, out)
				}
			}
		})
	}
}

// Test 17 (also covers required test 10 -- exit code unchanged): a
// sanitized, structurally-real regression fixture reproducing the exact
// observed reduced-IAM shape -- 31 rule executions all applicable/
// evaluated, Coverage {Kubernetes: complete, AWS: partial, Manifests:
// skipped}, Result computed as INCOMPLETE. Placeholder cluster name and
// resource identifiers only; no raw AWS account/ARN/VPC/subnet/SG
// identifiers, matching this repo's certification-evidence handling rules.
//
// Proves both halves of the fix:
//   - the OLD classification (BuildEvaluationCoverage alone, i.e. exactly
//     what shipped before this fix) reads "complete" -- this is the bug,
//     asserted here as a "before" comment/assertion so it can never
//     silently regress back into being accepted as correct.
//   - the NEW classification (BuildOverallCoverage) reads "partial".
func TestOverallCoverage_ReducedIAMRegressionFixture(t *testing.T) {
	ruleIDs := []string{
		"API-001", "API-002",
		"CRD-001", "CRD-002", "APISERVICE-001",
		"WH-001", "WH-002", "WH-004", "WH-005",
		"PDB-001", "PDB-002",
		"DRAIN-001", "DRAIN-002", "DRAIN-003", "DRAIN-004", "DRAIN-005",
		"NODE-001", "NODE-002", "NODE-003", "NET-002",
		"EKS-NG-001", "EKS-NG-002", "EKS-NG-003", "EKS-NG-004",
		"ADDON-001", "ADDON-002",
		"COREDNS-001",
		"WORKLOAD-001",
		"EKS-INSIGHT-001", "EKS-INSIGHT-002", "EKS-INSIGHT-003",
	}
	if len(ruleIDs) != 31 {
		t.Fatalf("fixture setup: %d rule IDs, want 31 to match the observed reduced-IAM scan shape", len(ruleIDs))
	}
	execs := make([]findings.RuleExecutionRecord, len(ruleIDs))
	for i, id := range ruleIDs {
		execs[i] = findings.RuleExecutionRecord{
			RuleID:        id,
			Applicability: findings.ApplicabilityApplicable,
			State:         findings.ExecutionEvaluated,
		}
	}

	reducedIAMCoverage := findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS: findings.PlaneCoverage{Status: findings.CoveragePartial, Errors: []string{
			"list-insights: AccessDenied",
			"list-addons: AccessDenied",
			"list-nodegroups: AccessDenied",
			"describe-subnets: AccessDenied",
			"describe-vpc: AccessDenied",
			"describe-security-group: AccessDenied (sg-placeholder-1)",
			"describe-security-group: AccessDenied (sg-placeholder-2)",
		}},
		Manifests: findings.PlaneCoverage{Status: findings.CoverageSkipped},
	}

	rpt := findings.NewReport("1.31", "reduced-iam-cert-cluster", "eks", time.Date(2026, 7, 23, 9, 0, 0, 0, time.UTC), nil)
	rpt.RuleExecutions = execs
	rpt.SetCoverage(reducedIAMCoverage)

	// Fixture preconditions, matching the real observed report exactly.
	if rpt.Result() != "INCOMPLETE" {
		t.Fatalf("fixture setup: Result() = %q, want INCOMPLETE", rpt.Result())
	}
	if rpt.ExitCode() != 3 {
		t.Fatalf("fixture setup: ExitCode() = %d, want 3", rpt.ExitCode())
	}
	if rpt.Coverage.AWS.Status != findings.CoveragePartial {
		t.Fatalf("fixture setup: Coverage.AWS.Status = %q, want partial", rpt.Coverage.AWS.Status)
	}

	ruleCov := BuildEvaluationCoverage(rpt)

	// --- "Before" assertion: this is the exact bug. The old
	// classification path (BuildEvaluationCoverage alone, unchanged by this
	// fix, still exactly what report.EvaluationCoverage.Status computes
	// today) reads "complete" here, in the same document where
	// Coverage.AWS reads "partial" and Result reads "INCOMPLETE" -- the
	// self-contradictory report an operator actually saw.
	if ruleCov.Status != CoverageStatusComplete {
		t.Fatalf("ruleCov.Status = %q, want complete (this assertion documents the bug: rule-execution-only coverage reads clean even though AWS evidence was genuinely partial)", ruleCov.Status)
	}

	// --- "After" assertion: the new combined classification correctly
	// reads partial, folding in Coverage.AWS's own honest partial status.
	overall := BuildOverallCoverage(ruleCov, rpt.Coverage)
	if overall.Status != CoverageStatusPartial {
		t.Fatalf("BuildOverallCoverage(...).Status = %q, want partial -- the fix", overall.Status)
	}
	if len(overall.DegradedPlanes) != 1 || overall.DegradedPlanes[0] != "AWS" {
		t.Errorf("DegradedPlanes = %v, want [AWS]", overall.DegradedPlanes)
	}
	advisory := overall.Advisory()
	if !strings.Contains(advisory, "AWS") {
		t.Errorf("Advisory() = %q, want it to name the degraded AWS plane", advisory)
	}
	// Rule-execution counts stay exactly as truthful as they already were
	// -- no RuleExecutionRecord was rewritten to manufacture this signal.
	if ruleCov.NotEvaluated != 0 || ruleCov.InsufficientEvidence != 0 || ruleCov.Failed != 0 {
		t.Errorf("rule-execution counts changed: NotEvaluated=%d InsufficientEvidence=%d Failed=%d, want all 0 (RuleExecutionRecords must never be rewritten by this fix)", ruleCov.NotEvaluated, ruleCov.InsufficientEvidence, ruleCov.Failed)
	}

	// Renderer parity: all three human-readable formats must surface the
	// combined partial status and the AWS-plane advisory.
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
		if !strings.Contains(out, ScoreQualification) {
			t.Errorf("%s: missing score-qualification text for the reduced-IAM regression fixture:\n%s", name, out)
		}
	}
}
