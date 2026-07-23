package cli

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
	"github.com/imneeteeshyadav98/kubepreflight/internal/rollback"
)

func TestRollbackCommandHasPlanAndAssess(t *testing.T) {
	exitCode := 0
	cmd := newRollbackCmd(&exitCode)

	if cmd.Name() != "rollback" {
		t.Fatalf("Name = %q, want rollback", cmd.Name())
	}
	for _, name := range []string{"plan", "assess"} {
		sub, _, err := cmd.Find([]string{name, "--help"})
		if err != nil {
			t.Fatalf("Find(%s): %v", name, err)
		}
		if sub == nil || sub.Name() != name {
			t.Fatalf("Find(%s) = %v", name, sub)
		}
		for _, flag := range []string{"provider", "cluster-name", "output", "assessment-out", "findings", "terminal-output", "collector-timeout"} {
			if sub.Flags().Lookup(flag) == nil {
				t.Fatalf("rollback %s missing --%s flag", name, flag)
			}
		}
	}
}

func TestRollbackReportTargetsAlwaysIncludeAssessmentJSON(t *testing.T) {
	targets := rollbackReportTargets("all", "out", "custom.json")
	got := targetPaths(targets)
	want := []string{
		filepath.Join("out", "custom.json"),
		filepath.Join("out", "rollback-report.md"),
		filepath.Join("out", "rollback-report.html"),
	}
	if len(got) != len(want) {
		t.Fatalf("targets = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("targets = %v, want %v", got, want)
		}
	}
}

func TestRollbackReportTargetsDoesNotDoublePrefixAnAlreadyJoinedAssessmentOut(t *testing.T) {
	// A caller that passes --output-dir and a matching --assessment-out
	// that already includes that same directory (the pattern
	// scripts/live-eks/run-smoke.sh uses, mirroring how scan's
	// --findings-out is invoked) must not get outputDir prepended twice.
	// Found via a real live EKS run: rollback plan failed writing to
	// out/out/rollback-assessment.json, a path that never exists.
	assessmentOut := filepath.Join("out", "rollback-assessment.json")
	targets := rollbackReportTargets("json", "out", assessmentOut)
	got := targetPaths(targets)
	want := []string{assessmentOut}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("targets = %v, want %v", got, want)
	}
}

func TestRollbackExitCodeMapping(t *testing.T) {
	tests := []struct {
		decision rollback.RecommendationDecision
		want     int
	}{
		{rollback.RecommendationRollbackPreferred, 0},
		{rollback.RecommendationFixForwardPreferred, 1},
		{rollback.RecommendationOperatorDecisionRequired, 1},
		{rollback.RecommendationDoNotProceed, 2},
	}
	for _, tc := range tests {
		got := rollbackExitCode(rollback.Assessment{
			Recommendation: rollback.Recommendation{Decision: tc.decision},
		})
		if got != tc.want {
			t.Fatalf("rollbackExitCode(%q) = %d, want %d", tc.decision, got, tc.want)
		}
	}
}

func TestRollbackExitCodeDisruptionOnlyFindingDoesNotReturnTwo(t *testing.T) {
	assessment := rollback.NewAssessment(rollback.ModePostUpgradeReadiness, time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC))
	assessment.Cluster = rollback.Cluster{
		Name:                  "prod",
		Provider:              "eks",
		CurrentVersion:        "1.35",
		RollbackTargetVersion: "1.34",
	}
	assessment.Eligibility = rollback.Eligibility{Status: rollback.EligibilityEligible, Source: "amazon-eks"}
	assessment.Readiness = rollback.Readiness{Status: rollback.ReadinessReady}
	assessment.Recommendation = rollback.Recommendation{Decision: rollback.RecommendationOperatorDecisionRequired, Confidence: rollback.ConfidenceMedium}
	assessment.Evidence = rollback.Evidence{Complete: true}

	report := findings.NewReport("1.34", "prod", "", time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC), []findings.Finding{{
		RuleID:       "PDB-001",
		Severity:     findings.SeverityBlocker,
		UpgradeGate:  findings.UpgradeGateBlock,
		ImpactScopes: []findings.ImpactScope{findings.ImpactScopeNodeDrain},
		Message:      "disruptionsAllowed=0 for forward worker rollout",
	}})
	report.CurrentVersion = "1.35"
	report.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageComplete},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageComplete},
	})

	got := rollback.ApplyRecommendation(rollback.ApplyOperationalReadiness(assessment, report))
	if got.Recommendation.Decision == rollback.RecommendationDoNotProceed {
		t.Fatalf("Recommendation = %q, want non-blocking decision without rollback disruption activation evidence", got.Recommendation.Decision)
	}
	if code := rollbackExitCode(got); code == 2 {
		t.Fatalf("rollbackExitCode = %d, want non-2 for disruption-only finding without activation evidence", code)
	}
}

func TestRollbackExitCodeMatchingAPIEvidenceTargetAllowsDoNotProceed(t *testing.T) {
	assessment := baseRollbackAssessment()

	report := findings.NewReport("1.34", "prod", "", time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC), []findings.Finding{{
		RuleID:   "API-001",
		Severity: findings.SeverityBlocker,
		Message:  "matching rollback target 1.34 removed API finding",
	}})
	report.CurrentVersion = "1.35"
	report.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageComplete},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageComplete},
	})

	got := rollback.ApplyRecommendation(rollback.ApplyOperationalReadiness(assessment, report))
	if got.Recommendation.Decision != rollback.RecommendationDoNotProceed {
		t.Fatalf("Recommendation = %q, want do_not_proceed for a genuine matching-target API-001 blocker", got.Recommendation.Decision)
	}
	if code := rollbackExitCode(got); code != 2 {
		t.Fatalf("rollbackExitCode = %d, want 2 for a genuine matching-target API-001 blocker", code)
	}
}

func TestRollbackExitCodeMismatchedAPIEvidenceTargetDoesNotReturnTwo(t *testing.T) {
	assessment := baseRollbackAssessment() // RollbackTargetVersion "1.34"

	report := findings.NewReport("1.36", "prod", "", time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC), []findings.Finding{{
		RuleID:   "API-001",
		Severity: findings.SeverityBlocker,
		Message:  "forward target 1.36 removed API finding, unrelated to rollback target 1.34",
	}})
	report.CurrentVersion = "1.35"
	report.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageComplete},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageComplete},
	})

	got := rollback.ApplyRecommendation(rollback.ApplyOperationalReadiness(assessment, report))
	if got.Recommendation.Decision == rollback.RecommendationDoNotProceed {
		t.Fatalf("Recommendation = %q, want mismatched API evidence target to not force do_not_proceed", got.Recommendation.Decision)
	}
	if code := rollbackExitCode(got); code == 2 {
		t.Fatalf("rollbackExitCode = %d, want non-2 for mismatched API evidence target alone", code)
	}
}

func baseRollbackAssessment() rollback.Assessment {
	assessment := rollback.NewAssessment(rollback.ModePostUpgradeReadiness, time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC))
	assessment.Cluster = rollback.Cluster{
		Name:                  "prod",
		Provider:              "eks",
		CurrentVersion:        "1.35",
		RollbackTargetVersion: "1.34",
	}
	assessment.Eligibility = rollback.Eligibility{Status: rollback.EligibilityEligible, Source: "amazon-eks"}
	assessment.Readiness = rollback.Readiness{Status: rollback.ReadinessReady}
	assessment.Recommendation = rollback.Recommendation{Decision: rollback.RecommendationOperatorDecisionRequired, Confidence: rollback.ConfidenceMedium}
	assessment.Evidence = rollback.Evidence{Complete: true}
	return assessment
}

func targetPaths(targets []rollbackReportTarget) []string {
	out := make([]string, 0, len(targets))
	for _, target := range targets {
		out = append(out, target.path)
	}
	return out
}
