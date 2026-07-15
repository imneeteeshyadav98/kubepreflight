package rollback

import (
	"testing"
	"time"

	"kubepreflight/internal/findings"
)

func TestApplyOperationalReadinessManagedNodegroupWarning(t *testing.T) {
	assessment := eligibleRollbackAssessment()
	report := cleanOperationalReport()
	report.EKSNodegroups = []findings.EKSNodegroupInfo{{
		Name:    "ng-app",
		Status:  "ACTIVE",
		Version: "1.35",
	}}

	got := ApplyOperationalReadiness(assessment, report)
	if got.Readiness.Status != ReadinessHighRisk {
		t.Fatalf("Readiness = %+v, want high risk", got.Readiness)
	}
	if got.Recommendation.Decision != assessment.Recommendation.Decision {
		t.Fatalf("Recommendation decision changed to %q, want untouched %q", got.Recommendation.Decision, assessment.Recommendation.Decision)
	}
	if !checkHasReason(got.Checks, "managed-nodegroups", ReasonManagedNodegroupRollbackRequired) {
		t.Fatalf("managed-nodegroups check missing reason: %+v", got.Checks)
	}
}

func TestApplyOperationalReadinessAddonBlocker(t *testing.T) {
	report := cleanOperationalReport()
	report.Findings = []findings.Finding{{
		RuleID:   "ADDON-001",
		Severity: findings.SeverityBlocker,
		Message:  "CoreDNS is incompatible with the rollback target.",
	}}

	got := ApplyOperationalReadiness(eligibleRollbackAssessment(), report)
	if got.Readiness.Status != ReadinessBlocked || got.Readiness.Blockers == 0 {
		t.Fatalf("Readiness = %+v, want blocked", got.Readiness)
	}
	if !checkHasReason(got.Checks, "managed-addons", ReasonManagedAddonRollbackRequired) {
		t.Fatalf("managed-addons check missing reason: %+v", got.Checks)
	}
}

func TestApplyOperationalReadinessPartialCoverageIsIncomplete(t *testing.T) {
	report := cleanOperationalReport()
	report.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoveragePartial, Errors: []string{"pods: forbidden"}},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageComplete},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageComplete},
	})

	got := ApplyOperationalReadiness(eligibleRollbackAssessment(), report)
	if got.Readiness.Status != ReadinessInsufficientEvidence {
		t.Fatalf("Readiness = %+v, want insufficient evidence", got.Readiness)
	}
	if got.Evidence.Complete {
		t.Fatal("Evidence.Complete = true, want false for partial coverage")
	}
	if !checkHasReason(got.Checks, "evidence-coverage", ReasonObservabilityEvidenceMissing) {
		t.Fatalf("evidence-coverage check missing reason: %+v", got.Checks)
	}
}

func TestApplyOperationalReadinessFindingFamilies(t *testing.T) {
	report := cleanOperationalReport()
	report.Findings = []findings.Finding{
		{RuleID: "WORKLOAD-001", Severity: findings.SeverityWarning, Message: "pod is CrashLoopBackOff"},
		{RuleID: "PDB-001", Severity: findings.SeverityBlocker, Message: "disruptionsAllowed=0"},
		{RuleID: "API-001", Severity: findings.SeverityBlocker, Message: "new-version-only API risk"},
		{RuleID: "WH-002", Severity: findings.SeverityWarning, Message: "webhook endpoint unavailable"},
	}

	got := ApplyOperationalReadiness(eligibleRollbackAssessment(), report)
	if got.Readiness.Status != ReadinessBlocked {
		t.Fatalf("Readiness = %+v, want blocked from PDB/API blockers", got.Readiness)
	}
	for _, tc := range []struct {
		id     string
		reason ReasonCode
	}{
		{"workload-health", ReasonUnhealthyWorkloadsPresent},
		{"disruption-readiness", ReasonPDBDisruptionConstraints},
		{"reverse-compatibility", ReasonNewVersionAPIAdoptionRisk},
		{"reverse-compatibility", ReasonCRDWebhookControllerRisk},
	} {
		if !checkHasReason(got.Checks, tc.id, tc.reason) {
			t.Fatalf("%s missing %s: %+v", tc.id, tc.reason, got.Checks)
		}
	}
}

func TestApplyOperationalReadinessNilReportIsIncomplete(t *testing.T) {
	got := ApplyOperationalReadiness(eligibleRollbackAssessment(), nil)
	if got.Readiness.Status != ReadinessInsufficientEvidence {
		t.Fatalf("Readiness = %+v, want insufficient evidence", got.Readiness)
	}
	if !checkHasReason(got.Checks, "operational-evidence", ReasonObservabilityEvidenceMissing) {
		t.Fatalf("operational-evidence check missing reason: %+v", got.Checks)
	}
}

func eligibleRollbackAssessment() Assessment {
	now := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	a := NewAssessment(ModePostUpgradeReadiness, now)
	a.Cluster = Cluster{
		Name:                  "prod",
		Region:                "ap-south-1",
		Provider:              "eks",
		CurrentVersion:        "1.35",
		RollbackTargetVersion: "1.34",
	}
	a.Eligibility = Eligibility{Status: EligibilityEligible, Source: "amazon-eks"}
	a.Readiness = Readiness{Status: ReadinessReady}
	a.Recommendation = Recommendation{Decision: RecommendationOperatorDecisionRequired, Confidence: ConfidenceMedium}
	a.Evidence = Evidence{Complete: true}
	return a
}

func cleanOperationalReport() *findings.Report {
	report := findings.NewReport("1.34", "prod", "eks", time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC), nil)
	report.CurrentVersion = "1.35"
	report.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageComplete},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageComplete},
	})
	report.EKSAddons = []findings.EKSAddonInfo{{Name: "coredns", CurrentVersion: "v1.12.0", Compatible: true}}
	return report
}

func checkHasReason(checks []Check, id string, reason ReasonCode) bool {
	for _, check := range checks {
		if check.ID != id {
			continue
		}
		for _, got := range check.ReasonCodes {
			if got == reason {
				return true
			}
		}
	}
	return false
}
