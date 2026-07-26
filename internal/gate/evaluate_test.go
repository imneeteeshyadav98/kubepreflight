package gate

import (
	"reflect"
	"testing"
	"time"

	"github.com/imneeteeshyadav98/kubepreflight/internal/comparison"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

func gateFinding(ruleID string, severity findings.Severity, name string) findings.Finding {
	ref := findings.LiveResource("PodDisruptionBudget", findings.ScopeNamespaced, "default", name, "uid-"+name)
	f := findings.Finding{
		RuleID:      ruleID,
		Severity:    severity,
		Confidence:  findings.TierStaticCertain,
		Message:     "gate test finding",
		Resources:   []findings.ResourceReference{ref},
		Fingerprint: findings.FingerprintV2(ruleID, "1.36", "", ref),
	}
	return findings.AssignPriority(f)
}

func gateReport(fs []findings.Finding) *findings.Report {
	r := findings.NewReport("1.36", "test-cluster", "", time.Now().UTC(), fs)
	r.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageSkipped},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageSkipped},
	})
	return r
}

func mustCompare(t *testing.T, baseline, current *findings.Report) *comparison.Comparison {
	t.Helper()
	cmp, err := comparison.Compare(baseline, current)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	return cmp
}

func TestEvaluate_CleanToCleanPasses(t *testing.T) {
	baseline, current := gateReport(nil), gateReport(nil)
	result := Evaluate(baseline, current, mustCompare(t, baseline, current), DefaultPolicy())

	if result.Decision != DecisionPass {
		t.Fatalf("Decision = %q, want pass: %+v", result.Decision, result)
	}
	if len(result.Reasons) != 0 {
		t.Errorf("Reasons = %v, want none", result.Reasons)
	}
}

func TestEvaluate_NewBlockerFails(t *testing.T) {
	blocker := gateFinding("PDB-001", findings.SeverityBlocker, "api")
	baseline, current := gateReport(nil), gateReport([]findings.Finding{blocker})
	result := Evaluate(baseline, current, mustCompare(t, baseline, current), DefaultPolicy())

	if result.Decision != DecisionFail {
		t.Fatalf("Decision = %q, want fail: %+v", result.Decision, result)
	}
	if !containsReason(result.Reasons, ReasonNewBlockersDetected) {
		t.Errorf("Reasons = %v, want NEW_BLOCKERS_DETECTED", result.Reasons)
	}
	if result.NewBlockers != 1 {
		t.Errorf("NewBlockers = %d, want 1", result.NewBlockers)
	}
}

func TestEvaluate_NewBlockerAllowedWhenPolicyDisabled(t *testing.T) {
	blocker := gateFinding("PDB-001", findings.SeverityBlocker, "api")
	baseline, current := gateReport(nil), gateReport([]findings.Finding{blocker})
	policy := DefaultPolicy()
	policy.FailOnNewBlockers = false
	policy.FailOnVerdictRegression = false
	policy.MinimumScoreDelta = -100 // isolate FailOnNewBlockers from the score-delta dimension
	result := Evaluate(baseline, current, mustCompare(t, baseline, current), policy)

	if result.Decision != DecisionPass {
		t.Fatalf("Decision = %q, want pass when FailOnNewBlockers is disabled: %+v", result.Decision, result)
	}
}

func TestEvaluate_WarningPolicyIgnoreNeverFailsOnWarnings(t *testing.T) {
	warning := gateFinding("WH-002", findings.SeverityWarning, "guard")
	baseline, current := gateReport(nil), gateReport([]findings.Finding{warning})
	policy := DefaultPolicy()
	policy.FailOnVerdictRegression = false // CLEAN -> PASSED_WITH_WARNINGS would otherwise fail
	policy.MinimumScoreDelta = -100        // isolate WarningPolicy from the score-delta dimension
	result := Evaluate(baseline, current, mustCompare(t, baseline, current), policy)

	if result.Decision != DecisionPass {
		t.Fatalf("Decision = %q, want pass under WarningPolicyIgnore: %+v", result.Decision, result)
	}
}

func TestEvaluate_WarningPolicyFailOnNewFailsOnNewWarningOnly(t *testing.T) {
	existing := gateFinding("WH-002", findings.SeverityWarning, "old-guard")
	newWarning := gateFinding("WH-002", findings.SeverityWarning, "new-guard")

	t.Run("new warning fails", func(t *testing.T) {
		baseline, current := gateReport(nil), gateReport([]findings.Finding{newWarning})
		policy := DefaultPolicy()
		policy.WarningPolicy = WarningPolicyFailOnNew
		policy.FailOnVerdictRegression = false
		result := Evaluate(baseline, current, mustCompare(t, baseline, current), policy)

		if result.Decision != DecisionFail {
			t.Fatalf("Decision = %q, want fail: %+v", result.Decision, result)
		}
		if !containsReason(result.Reasons, ReasonNewWarningsDetected) {
			t.Errorf("Reasons = %v, want NEW_WARNINGS_DETECTED", result.Reasons)
		}
	})

	t.Run("only pre-existing warning does not fail", func(t *testing.T) {
		baseline, current := gateReport([]findings.Finding{existing}), gateReport([]findings.Finding{existing})
		policy := DefaultPolicy()
		policy.WarningPolicy = WarningPolicyFailOnNew
		result := Evaluate(baseline, current, mustCompare(t, baseline, current), policy)

		if result.Decision != DecisionPass {
			t.Fatalf("Decision = %q, want pass (warning is unchanged, not new): %+v", result.Decision, result)
		}
	})
}

func TestEvaluate_WarningPolicyFailOnAnyFailsOnPreExistingWarning(t *testing.T) {
	existing := gateFinding("WH-002", findings.SeverityWarning, "old-guard")
	baseline, current := gateReport([]findings.Finding{existing}), gateReport([]findings.Finding{existing})
	policy := DefaultPolicy()
	policy.WarningPolicy = WarningPolicyFailOnAny
	result := Evaluate(baseline, current, mustCompare(t, baseline, current), policy)

	if result.Decision != DecisionFail {
		t.Fatalf("Decision = %q, want fail under WarningPolicyFailOnAny even for an unchanged warning: %+v", result.Decision, result)
	}
	if !containsReason(result.Reasons, ReasonWarningsPresent) {
		t.Errorf("Reasons = %v, want WARNINGS_PRESENT", result.Reasons)
	}
	if result.CurrentWarnings != 1 {
		t.Errorf("CurrentWarnings = %d, want 1", result.CurrentWarnings)
	}
}

func TestEvaluate_VerdictRegressionFails(t *testing.T) {
	blocker := gateFinding("PDB-001", findings.SeverityBlocker, "api")
	baseline, current := gateReport(nil), gateReport([]findings.Finding{blocker})
	policy := DefaultPolicy()
	policy.FailOnNewBlockers = false // isolate the verdict-regression reason
	result := Evaluate(baseline, current, mustCompare(t, baseline, current), policy)

	if result.Decision != DecisionFail {
		t.Fatalf("Decision = %q, want fail: %+v", result.Decision, result)
	}
	if !containsReason(result.Reasons, ReasonReadinessVerdictRegressed) {
		t.Errorf("Reasons = %v, want READINESS_VERDICT_REGRESSED", result.Reasons)
	}
}

func TestEvaluate_VerdictImprovementNeverFails(t *testing.T) {
	blocker := gateFinding("PDB-001", findings.SeverityBlocker, "api")
	baseline, current := gateReport([]findings.Finding{blocker}), gateReport(nil)
	result := Evaluate(baseline, current, mustCompare(t, baseline, current), DefaultPolicy())

	if result.Decision != DecisionPass {
		t.Fatalf("Decision = %q, want pass (BLOCKED -> CLEAN is an improvement): %+v", result.Decision, result)
	}
}

func TestEvaluate_ScoreRegressionBelowMinimumFails(t *testing.T) {
	blocker := gateFinding("PDB-001", findings.SeverityBlocker, "api")
	baseline, current := gateReport(nil), gateReport([]findings.Finding{blocker})
	policy := DefaultPolicy()
	policy.FailOnNewBlockers = false
	policy.FailOnVerdictRegression = false
	policy.MinimumScoreDelta = 0
	result := Evaluate(baseline, current, mustCompare(t, baseline, current), policy)

	if result.Decision != DecisionFail {
		t.Fatalf("Decision = %q, want fail (score dropped below MinimumScoreDelta=0): %+v", result.Decision, result)
	}
	if !containsReason(result.Reasons, ReasonReadinessScoreRegressed) {
		t.Errorf("Reasons = %v, want READINESS_SCORE_REGRESSED", result.Reasons)
	}
	if result.ScoreDelta >= 0 {
		t.Errorf("ScoreDelta = %d, want negative", result.ScoreDelta)
	}
}

func TestEvaluate_ToleratedScoreDeltaPasses(t *testing.T) {
	blocker := gateFinding("PDB-001", findings.SeverityBlocker, "api")
	baseline, current := gateReport(nil), gateReport([]findings.Finding{blocker})
	policy := DefaultPolicy()
	policy.FailOnNewBlockers = false
	policy.FailOnVerdictRegression = false
	policy.MinimumScoreDelta = -100 // tolerate any drop
	result := Evaluate(baseline, current, mustCompare(t, baseline, current), policy)

	if result.Decision != DecisionPass {
		t.Fatalf("Decision = %q, want pass when MinimumScoreDelta tolerates the drop: %+v", result.Decision, result)
	}
}

func TestEvaluate_IncompleteBaselineIsNeutral(t *testing.T) {
	baseline := gateReport(nil)
	baseline.SetCoverage(findings.ScanCoverage{Kubernetes: findings.PlaneCoverage{Status: findings.CoveragePartial}})
	current := gateReport(nil)
	result := Evaluate(baseline, current, mustCompare(t, baseline, current), DefaultPolicy())

	if result.Decision != DecisionNeutral {
		t.Fatalf("Decision = %q, want neutral for an incomplete baseline: %+v", result.Decision, result)
	}
	if !containsReason(result.Reasons, ReasonInsufficientEvidence) {
		t.Errorf("Reasons = %v, want INSUFFICIENT_EVIDENCE", result.Reasons)
	}
}

func TestEvaluate_IncompleteCurrentIsNeutral(t *testing.T) {
	baseline := gateReport(nil)
	current := gateReport(nil)
	current.SetCoverage(findings.ScanCoverage{Kubernetes: findings.PlaneCoverage{Status: findings.CoveragePartial}})
	result := Evaluate(baseline, current, mustCompare(t, baseline, current), DefaultPolicy())

	if result.Decision != DecisionNeutral {
		t.Fatalf("Decision = %q, want neutral for an incomplete current scan: %+v", result.Decision, result)
	}
	if !containsReason(result.Reasons, ReasonInsufficientEvidence) {
		t.Errorf("Reasons = %v, want INSUFFICIENT_EVIDENCE", result.Reasons)
	}
}

func TestEvaluate_TargetVersionMismatchIsNeutral(t *testing.T) {
	baseline := findings.NewReport("1.35", "test", "", time.Now(), nil)
	baseline.SetCoverage(findings.ScanCoverage{Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete}})
	current := findings.NewReport("1.36", "test", "", time.Now(), nil)
	current.SetCoverage(findings.ScanCoverage{Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete}})

	result := Evaluate(baseline, current, mustCompare(t, baseline, current), DefaultPolicy())

	if result.Decision != DecisionNeutral {
		t.Fatalf("Decision = %q, want neutral for a target-version-mismatched comparison: %+v", result.Decision, result)
	}
	if !containsReason(result.Reasons, ReasonInsufficientEvidence) {
		t.Errorf("Reasons = %v, want INSUFFICIENT_EVIDENCE", result.Reasons)
	}
}

func TestEvaluate_MultipleFailureReasonsAreStableOrdered(t *testing.T) {
	blocker := gateFinding("PDB-001", findings.SeverityBlocker, "api")
	warning := gateFinding("WH-002", findings.SeverityWarning, "guard")
	baseline, current := gateReport(nil), gateReport([]findings.Finding{blocker, warning})
	policy := DefaultPolicy()
	policy.WarningPolicy = WarningPolicyFailOnNew
	policy.MinimumScoreDelta = 0

	result := Evaluate(baseline, current, mustCompare(t, baseline, current), policy)

	want := []ReasonCode{
		ReasonNewBlockersDetected,
		ReasonNewWarningsDetected,
		ReasonReadinessVerdictRegressed,
		ReasonReadinessScoreRegressed,
	}
	if !reflect.DeepEqual(result.Reasons, want) {
		t.Fatalf("Reasons = %v, want %v in stable policy-field order", result.Reasons, want)
	}

	// Running it again must produce byte-identical ordering -- the gate's
	// whole point is a deterministic CI decision, not one that could flap
	// between runs of the same inputs.
	again := Evaluate(baseline, current, mustCompare(t, baseline, current), policy)
	if !reflect.DeepEqual(again.Reasons, want) {
		t.Fatalf("second Evaluate() Reasons = %v, want identical %v", again.Reasons, want)
	}
}

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()
	if !p.FailOnNewBlockers {
		t.Error("FailOnNewBlockers = false, want true")
	}
	if p.WarningPolicy != WarningPolicyIgnore {
		t.Errorf("WarningPolicy = %q, want ignore", p.WarningPolicy)
	}
	if !p.FailOnVerdictRegression {
		t.Error("FailOnVerdictRegression = false, want true")
	}
	if p.MinimumScoreDelta != 0 {
		t.Errorf("MinimumScoreDelta = %d, want 0", p.MinimumScoreDelta)
	}
}

// TestEvaluate_NotReEvaluatedNeverCountsAsResolved covers test 18: a
// baseline-only blocker whose rule was not re-evaluated in current (e.g. a
// manifests-only rescan) must never be counted in the gate's
// ResolvedFindings -- comparison.Compare (see internal/comparison/compare.go)
// now buckets it NotReEvaluated instead of Resolved, and Evaluate must
// simply reflect that smaller, correct Resolved count rather than needing
// any policy change of its own (gate *policy* for this bucket is PR 6's
// job, not this PR's -- see docs/roadmap/v1.3.0-scope-audit.md's PR 5/PR 6
// split). This also confirms every other gate-decision dimension (new
// blockers, warnings, verdict regression, score delta) behaves identically
// to before for this non-not_re_evaluated-driven scenario.
func TestEvaluate_NotReEvaluatedNeverCountsAsResolved(t *testing.T) {
	blocker := gateFinding("PDB-001", findings.SeverityBlocker, "api")
	baseline := gateReport([]findings.Finding{blocker})
	current := gateReport(nil)
	current.RuleExecutions = []findings.RuleExecutionRecord{
		{RuleID: "PDB-001", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionNotEvaluated},
	}

	cmp := mustCompare(t, baseline, current)
	if cmp.Summary.Resolved != 0 {
		t.Fatalf("cmp.Summary.Resolved = %d, want 0 -- the rule was never proven to have re-evaluated", cmp.Summary.Resolved)
	}
	if cmp.Summary.NotReEvaluated != 1 {
		t.Fatalf("cmp.Summary.NotReEvaluated = %d, want 1", cmp.Summary.NotReEvaluated)
	}

	result := Evaluate(baseline, current, cmp, DefaultPolicy())
	if result.ResolvedFindings != 0 {
		t.Errorf("ResolvedFindings = %d, want 0 -- a not_re_evaluated finding must never be silently folded into the gate's resolved count", result.ResolvedFindings)
	}
	// Everything else about this scenario behaves exactly as it would have
	// pre-PR-5: no new blockers, no warnings, verdict improved (not
	// regressed) BLOCKED -> CLEAN, non-negative score delta -- so the gate
	// still passes.
	if result.Decision != DecisionPass {
		t.Fatalf("Decision = %q, want pass: %+v", result.Decision, result)
	}
	if len(result.Reasons) != 0 {
		t.Errorf("Reasons = %v, want none", result.Reasons)
	}
}

func containsReason(reasons []ReasonCode, want ReasonCode) bool {
	for _, r := range reasons {
		if r == want {
			return true
		}
	}
	return false
}
