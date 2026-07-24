package eks

import (
	"context"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
	"github.com/imneeteeshyadav98/kubepreflight/internal/rollback"
)

// This file exercises the real, full rollback pipeline -- EvaluateEligibility
// -> ApplyRollbackInsights -> rollback.ApplyOperationalReadiness ->
// rollback.ApplyRecommendation -> Assessment.Validate() -- against synthetic
// (no-network, no-AWS-credential) snapshots that reproduce a definitive
// provider eligibility failure (expired rollback window, unsupported
// rollback target) with no or imperfect --findings evidence.
//
// Before the fix in internal/rollback/model.go, this shape reliably fails
// model validation: EvaluateEligibility marks the hard blocker's Readiness as
// "blocked", but ApplyRollbackInsights (when the snapshot has no rollback
// insights of its own) unconditionally recomputes Readiness/Recommendation
// from insight evidence alone (see markInsightsUnavailable and the
// remainingUnknownEvidence branch in insights.go), and
// rollback.combineReadiness's "existing == ready collapses to operational"
// rule then lets a subsequent nil/imperfect --findings evaluation downgrade
// the final Readiness.Status to insufficient_evidence. Eligibility.Status
// stays "unavailable" throughout, so rollback.ApplyRecommendation still
// (correctly) produces do_not_proceed/high -- but the pre-fix Validate()
// guard rejected every insufficient_evidence + high-confidence combination
// outright, regardless of Decision, so this authoritative stop failed to
// build a valid assessment.

func expiredRollbackWindowAssessment(t *testing.T, now time.Time) rollback.Assessment {
	t.Helper()
	client := healthyFakeClient(now.Add(-(8 * 24) * time.Hour)) // 8 days > the 7-day rollback window
	snap, err := NewCollector(client, "prod", "ap-south-1").Collect(context.Background(), time.Second, now)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	assessment := EvaluateEligibility(snap, now)
	if assessment.Eligibility.Status != rollback.EligibilityUnavailable {
		t.Fatalf("fixture Eligibility = %q, want unavailable (expired window)", assessment.Eligibility.Status)
	}
	if !hasReason(assessment.Eligibility.ReasonCodes, rollback.ReasonRollbackWindowExpired) {
		t.Fatalf("fixture ReasonCodes = %v, want rollback window expired", assessment.Eligibility.ReasonCodes)
	}
	return ApplyRollbackInsights(assessment, snap, now)
}

func unsupportedRollbackTargetAssessment(t *testing.T, now time.Time) rollback.Assessment {
	t.Helper()
	client := healthyFakeClient(now.Add(-24 * time.Hour))
	client.describeClusterVersionsOut.ClusterVersions = []ekstypes.ClusterVersionInformation{
		{ClusterVersion: awssdk.String("1.34"), VersionStatus: ekstypes.VersionStatusUnsupported},
	}
	snap, err := NewCollector(client, "prod", "ap-south-1").Collect(context.Background(), time.Second, now)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	assessment := EvaluateEligibility(snap, now)
	if assessment.Eligibility.Status != rollback.EligibilityUnavailable {
		t.Fatalf("fixture Eligibility = %q, want unavailable (unsupported target)", assessment.Eligibility.Status)
	}
	if !hasReason(assessment.Eligibility.ReasonCodes, rollback.ReasonRollbackTargetUnsupported) {
		t.Fatalf("fixture ReasonCodes = %v, want rollback target unsupported", assessment.Eligibility.ReasonCodes)
	}
	return ApplyRollbackInsights(assessment, snap, now)
}

// assertHardBlockerContract is the common assertion set every test in this
// file proves: eligibility, readiness, and the final decision layer must all
// agree, and the resulting document must be a structurally valid assessment
// (Validate() == nil) -- the exact invariant that was broken before this fix.
func assertHardBlockerContract(t *testing.T, got rollback.Assessment) {
	t.Helper()
	if got.Eligibility.Status != rollback.EligibilityUnavailable {
		t.Fatalf("Eligibility = %q, want unavailable", got.Eligibility.Status)
	}
	if got.Recommendation.Decision != rollback.RecommendationDoNotProceed {
		t.Fatalf("Decision = %q, want do_not_proceed", got.Recommendation.Decision)
	}
	if got.Recommendation.Confidence != rollback.ConfidenceHigh {
		t.Fatalf("Confidence = %q, want high", got.Recommendation.Confidence)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want a valid assessment", err)
	}
}

// TestExpiredRollbackWindowNoFindingsProducesValidDoNotProceed is test 4: an
// expired rollback window with no --findings supplied at all.
func TestExpiredRollbackWindowNoFindingsProducesValidDoNotProceed(t *testing.T) {
	now := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	assessment := expiredRollbackWindowAssessment(t, now)

	got := rollback.ApplyRecommendation(rollback.ApplyOperationalReadiness(assessment, nil))

	if got.Readiness.Status != rollback.ReadinessInsufficientEvidence {
		t.Fatalf("Readiness = %q, want insufficient_evidence (reproduces the pre-fix failure shape) -- got %+v", got.Readiness.Status, got.Readiness)
	}
	assertHardBlockerContract(t, got)
}

// TestUnsupportedRollbackTargetNoFindingsProducesValidDoNotProceed is test 5:
// an unsupported rollback target (a different authoritative eligibility
// failure than the expired window) with no --findings supplied.
func TestUnsupportedRollbackTargetNoFindingsProducesValidDoNotProceed(t *testing.T) {
	now := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	assessment := unsupportedRollbackTargetAssessment(t, now)

	got := rollback.ApplyRecommendation(rollback.ApplyOperationalReadiness(assessment, nil))

	if got.Readiness.Status != rollback.ReadinessInsufficientEvidence {
		t.Fatalf("Readiness = %q, want insufficient_evidence -- got %+v", got.Readiness.Status, got.Readiness)
	}
	assertHardBlockerContract(t, got)
}

// TestExpiredRollbackWindowWithCleanFindingsRemainsValid is test 6: confirms
// the pre-existing, already-working path is unchanged by this fix -- a hard
// eligibility blocker combined with clean, matching, fresh findings produces
// Readiness "ready" (not insufficient_evidence), so this shape validated
// successfully even before the fix.
func TestExpiredRollbackWindowWithCleanFindingsRemainsValid(t *testing.T) {
	now := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	assessment := expiredRollbackWindowAssessment(t, now)

	report := findings.NewReport("1.34", "prod", "eks", now, nil)
	report.CurrentVersion = "1.35"
	report.EKSCluster = &findings.EKSClusterInfo{ClusterName: "prod", Region: "ap-south-1"}
	report.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageComplete},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageComplete},
	})

	got := rollback.ApplyRecommendation(rollback.ApplyOperationalReadiness(assessment, report))

	if got.Readiness.Status != rollback.ReadinessReady {
		t.Fatalf("Readiness = %q, want ready for clean matching fresh findings", got.Readiness.Status)
	}
	assertHardBlockerContract(t, got)
}

// TestExpiredRollbackWindowWithStaleFindingsRemainsValid is test 7: the
// PR #209 findings-freshness gate routes cluster-specific checks to unknown
// when the supplied findings.json is older than the 24-hour maximum age.
// That must still leave the authoritative eligibility blocker's
// do_not_proceed/high untouched, and the resulting assessment valid.
func TestExpiredRollbackWindowWithStaleFindingsRemainsValid(t *testing.T) {
	now := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	assessment := expiredRollbackWindowAssessment(t, now)

	report := findings.NewReport("1.34", "prod", "eks", now.Add(-48*time.Hour), nil) // stale: 48h > 24h max age
	report.CurrentVersion = "1.35"
	report.EKSCluster = &findings.EKSClusterInfo{ClusterName: "prod", Region: "ap-south-1"} // matching cluster
	report.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageComplete},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageComplete},
	})

	got := rollback.ApplyRecommendation(rollback.ApplyOperationalReadiness(assessment, report))

	if got.Readiness.Status != rollback.ReadinessInsufficientEvidence {
		t.Fatalf("Readiness = %q, want insufficient_evidence from stale findings", got.Readiness.Status)
	}
	if !hasRollbackCheckReason(got.Checks, rollback.ReasonRollbackEvidenceStale) {
		t.Fatalf("Checks = %+v, want a stale-evidence reason code preserved", got.Checks)
	}
	assertHardBlockerContract(t, got)
}

// TestExpiredRollbackWindowWithWrongClusterFindingsRemainsValid is test 8:
// the PR #208 cluster-identity gate routes cluster-specific checks to
// unknown when the supplied findings.json's live EKS cluster name doesn't
// match the assessed cluster. The authoritative eligibility blocker must
// still win.
func TestExpiredRollbackWindowWithWrongClusterFindingsRemainsValid(t *testing.T) {
	now := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	assessment := expiredRollbackWindowAssessment(t, now)

	report := findings.NewReport("1.34", "staging", "eks", now, nil)
	report.CurrentVersion = "1.35"
	report.EKSCluster = &findings.EKSClusterInfo{ClusterName: "staging", Region: "ap-south-1"} // wrong cluster name
	report.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageComplete},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageComplete},
	})

	got := rollback.ApplyRecommendation(rollback.ApplyOperationalReadiness(assessment, report))

	if got.Readiness.Status != rollback.ReadinessInsufficientEvidence {
		t.Fatalf("Readiness = %q, want insufficient_evidence from wrong-cluster findings", got.Readiness.Status)
	}
	if !hasRollbackCheckReason(got.Checks, rollback.ReasonRollbackEvidenceClusterMismatch) {
		t.Fatalf("Checks = %+v, want a cluster-mismatch reason code preserved", got.Checks)
	}
	assertHardBlockerContract(t, got)
}

// TestExpiredRollbackWindowWithWrongTargetFindingsRemainsValid is test 9: the
// PR #207 API-evidence-target gate routes API-001/API-002 findings to
// unknown when findings.json's targetVersion doesn't match the rollback
// target. The authoritative eligibility blocker must still win.
func TestExpiredRollbackWindowWithWrongTargetFindingsRemainsValid(t *testing.T) {
	now := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	assessment := expiredRollbackWindowAssessment(t, now) // RollbackTargetVersion "1.34"

	report := findings.NewReport("1.36", "prod", "eks", now, []findings.Finding{{
		RuleID:   "API-001",
		Severity: findings.SeverityBlocker,
		Message:  "forward target 1.36 removed API finding, unrelated to rollback target 1.34",
	}})
	report.CurrentVersion = "1.35"
	report.EKSCluster = &findings.EKSClusterInfo{ClusterName: "prod", Region: "ap-south-1"} // matching cluster
	report.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageComplete},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageComplete},
	})

	got := rollback.ApplyRecommendation(rollback.ApplyOperationalReadiness(assessment, report))

	if got.Readiness.Status != rollback.ReadinessInsufficientEvidence {
		t.Fatalf("Readiness = %q, want insufficient_evidence from mismatched API evidence target", got.Readiness.Status)
	}
	if !hasRollbackCheckReason(got.Checks, rollback.ReasonRollbackEvidenceTargetMismatch) {
		t.Fatalf("Checks = %+v, want an evidence-target-mismatch reason code preserved", got.Checks)
	}
	assertHardBlockerContract(t, got)
}

// TestEligibleRollbackWithInsufficientEvidenceIsNotHighConfidenceApproval is
// test 10: the negative-space proof that this exemption is narrowly scoped
// to a confirmed hard STOP. When eligibility is NOT unavailable (here,
// eligible and within the rollback window) and readiness is only
// insufficient_evidence (no --findings, no rollback insights refreshed
// recently enough), the real pipeline must never produce a high-confidence
// rollback_preferred approval.
func TestEligibleRollbackWithInsufficientEvidenceIsNotHighConfidenceApproval(t *testing.T) {
	now := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	snap, err := NewCollector(healthyFakeClient(now.Add(-24*time.Hour)), "prod", "ap-south-1").Collect(context.Background(), time.Second, now)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	assessment := EvaluateEligibility(snap, now)
	if assessment.Eligibility.Status != rollback.EligibilityEligible {
		t.Fatalf("fixture Eligibility = %q, want eligible", assessment.Eligibility.Status)
	}
	assessment = ApplyRollbackInsights(assessment, snap, now)

	got := rollback.ApplyRecommendation(rollback.ApplyOperationalReadiness(assessment, nil))

	if got.Readiness.Status != rollback.ReadinessInsufficientEvidence {
		t.Fatalf("Readiness = %q, want insufficient_evidence", got.Readiness.Status)
	}
	if got.Recommendation.Confidence == rollback.ConfidenceHigh {
		t.Fatalf("Confidence = %q, insufficient evidence must never justify high confidence", got.Recommendation.Confidence)
	}
	if got.Recommendation.Decision == rollback.RecommendationRollbackPreferred {
		t.Fatalf("Decision = %q, insufficient evidence must never justify rollback_preferred", got.Recommendation.Decision)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func hasRollbackCheckReason(checks []rollback.Check, want rollback.ReasonCode) bool {
	for _, check := range checks {
		for _, reason := range check.ReasonCodes {
			if reason == want {
				return true
			}
		}
	}
	return false
}
