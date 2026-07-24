package rollback

import (
	"reflect"
	"testing"
	"time"
)

func TestApplyRecommendationUnavailableEligibilityStopsRollback(t *testing.T) {
	assessment := recommendationBase()
	assessment.Eligibility.Status = EligibilityUnavailable
	assessment.Eligibility.ReasonCodes = []ReasonCode{ReasonRollbackWindowExpired}
	assessment.Readiness = Readiness{Status: ReadinessReady}
	assessment.Evidence.Complete = true

	got := ApplyRecommendation(assessment)

	if got.Recommendation.Decision != RecommendationDoNotProceed {
		t.Fatalf("Decision = %q, want %q", got.Recommendation.Decision, RecommendationDoNotProceed)
	}
	if got.Recommendation.Confidence != ConfidenceHigh {
		t.Fatalf("Confidence = %q, want %q", got.Recommendation.Confidence, ConfidenceHigh)
	}
	if !hasRecommendationReason(got, ReasonRollbackWindowExpired) {
		t.Fatalf("ReasonCodes = %v, want rollback window expired", got.Recommendation.ReasonCodes)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

// TestApplyRecommendationUnavailableEligibilityWithInsufficientEvidenceStillStopsRollback
// is the real-execution shape this fix targets: EvaluateEligibility's own
// "blocked" readiness for a hard eligibility failure gets overwritten by
// ApplyRollbackInsights/ApplyOperationalReadiness down to
// insufficient_evidence when no rollback insights or --findings evidence is
// available (see internal/rollback/eks's markInsightsUnavailable and
// combineReadiness's Ready-collapses-to-operational path). ApplyRecommendation
// must still stop the rollback with high confidence purely from
// Eligibility.Status == unavailable, and the resulting assessment must pass
// Validate() -- proving recommendation generation and model validation now
// agree (see model_test.go's
// TestAssessmentValidateAllowsHighConfidenceDoNotProceedWithInsufficientEvidence).
func TestApplyRecommendationUnavailableEligibilityWithInsufficientEvidenceStillStopsRollback(t *testing.T) {
	assessment := recommendationBase()
	assessment.Eligibility.Status = EligibilityUnavailable
	assessment.Eligibility.ReasonCodes = []ReasonCode{ReasonRollbackWindowExpired}
	assessment.Readiness = Readiness{Status: ReadinessInsufficientEvidence, Unknowns: 1}
	assessment.Evidence.Complete = false

	got := ApplyRecommendation(assessment)

	if got.Recommendation.Decision != RecommendationDoNotProceed {
		t.Fatalf("Decision = %q, want %q", got.Recommendation.Decision, RecommendationDoNotProceed)
	}
	if got.Recommendation.Confidence != ConfidenceHigh {
		t.Fatalf("Confidence = %q, want %q", got.Recommendation.Confidence, ConfidenceHigh)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want hard eligibility blocker + insufficient evidence to remain a valid assessment", err)
	}
}

func TestApplyRecommendationUnknownEligibilityRequiresOperatorDecision(t *testing.T) {
	assessment := recommendationBase()
	assessment.Eligibility.Status = EligibilityUnknown
	assessment.Eligibility.ReasonCodes = []ReasonCode{ReasonEKSUpgradeHistoryUnavailable}
	assessment.Readiness = Readiness{Status: ReadinessReady}
	assessment.Evidence.Complete = true

	got := ApplyRecommendation(assessment)

	if got.Recommendation.Decision != RecommendationOperatorDecisionRequired {
		t.Fatalf("Decision = %q, want %q", got.Recommendation.Decision, RecommendationOperatorDecisionRequired)
	}
	if got.Recommendation.Confidence != ConfidenceLow {
		t.Fatalf("Confidence = %q, want %q", got.Recommendation.Confidence, ConfidenceLow)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestApplyRecommendationBlockedReadinessStopsRollback(t *testing.T) {
	assessment := recommendationBase()
	assessment.Readiness = Readiness{Status: ReadinessBlocked, Blockers: 1}
	assessment.Checks = []Check{{
		ID:          "rollback-insight-upgrade-readiness",
		Status:      CheckFail,
		ReasonCodes: []ReasonCode{ReasonEKSInsightsBlocking},
	}}
	assessment.Evidence.Complete = true

	got := ApplyRecommendation(assessment)

	if got.Recommendation.Decision != RecommendationDoNotProceed {
		t.Fatalf("Decision = %q, want %q", got.Recommendation.Decision, RecommendationDoNotProceed)
	}
	if got.Recommendation.Confidence != ConfidenceHigh {
		t.Fatalf("Confidence = %q, want %q", got.Recommendation.Confidence, ConfidenceHigh)
	}
	if !hasRecommendationReason(got, ReasonEKSInsightsBlocking) {
		t.Fatalf("ReasonCodes = %v, want insights blocking", got.Recommendation.ReasonCodes)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestApplyRecommendationInsufficientEvidenceRequiresOperatorDecision(t *testing.T) {
	assessment := recommendationBase()
	assessment.Readiness = Readiness{Status: ReadinessInsufficientEvidence, Unknowns: 1}
	assessment.Evidence.Complete = false
	assessment.Checks = []Check{{
		ID:          "evidence-coverage",
		Status:      CheckUnknown,
		ReasonCodes: []ReasonCode{ReasonObservabilityEvidenceMissing},
	}}

	got := ApplyRecommendation(assessment)

	if got.Recommendation.Decision != RecommendationOperatorDecisionRequired {
		t.Fatalf("Decision = %q, want %q", got.Recommendation.Decision, RecommendationOperatorDecisionRequired)
	}
	if got.Recommendation.Confidence != ConfidenceLow {
		t.Fatalf("Confidence = %q, want %q", got.Recommendation.Confidence, ConfidenceLow)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestApplyRecommendationHighRiskPrefersFixForward(t *testing.T) {
	assessment := recommendationBase()
	assessment.Readiness = Readiness{Status: ReadinessHighRisk, Warnings: 2}
	assessment.Evidence.Complete = true
	assessment.Checks = []Check{{
		ID:          "managed-nodegroups",
		Status:      CheckWarning,
		ReasonCodes: []ReasonCode{ReasonManagedNodegroupRollbackRequired},
	}}

	got := ApplyRecommendation(assessment)

	if got.Recommendation.Decision != RecommendationFixForwardPreferred {
		t.Fatalf("Decision = %q, want %q", got.Recommendation.Decision, RecommendationFixForwardPreferred)
	}
	if got.Recommendation.Confidence != ConfidenceMedium {
		t.Fatalf("Confidence = %q, want %q", got.Recommendation.Confidence, ConfidenceMedium)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestApplyRecommendationReadyCompletePrefersRollback(t *testing.T) {
	assessment := recommendationBase()
	assessment.Readiness = Readiness{Status: ReadinessReady}
	assessment.Evidence.Complete = true

	got := ApplyRecommendation(assessment)

	if got.Recommendation.Decision != RecommendationRollbackPreferred {
		t.Fatalf("Decision = %q, want %q", got.Recommendation.Decision, RecommendationRollbackPreferred)
	}
	if got.Recommendation.Confidence != ConfidenceMedium {
		t.Fatalf("Confidence = %q, want %q", got.Recommendation.Confidence, ConfidenceMedium)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestApplyRecommendationReadyIncompleteRequiresOperatorDecision(t *testing.T) {
	assessment := recommendationBase()
	assessment.Readiness = Readiness{Status: ReadinessReady}
	assessment.Evidence.Complete = false

	got := ApplyRecommendation(assessment)

	if got.Recommendation.Decision != RecommendationOperatorDecisionRequired {
		t.Fatalf("Decision = %q, want %q", got.Recommendation.Decision, RecommendationOperatorDecisionRequired)
	}
	if got.Recommendation.Confidence != ConfidenceLow {
		t.Fatalf("Confidence = %q, want %q", got.Recommendation.Confidence, ConfidenceLow)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

// TestApplyRecommendationEligibleInsufficientEvidenceNeverHighConfidenceApproval
// is the critical negative-space test proving the do_not_proceed exemption in
// Validate() is correctly scoped: when eligibility is NOT unavailable (here,
// eligible) and readiness is merely insufficient_evidence, ApplyRecommendation
// must never produce a high-confidence rollback_preferred approval. This
// mirrors TestApplyRecommendationInsufficientEvidenceRequiresOperatorDecision's
// fixture but asserts the confidence/decision safety properties explicitly by
// name, as its own documented regression guard.
func TestApplyRecommendationEligibleInsufficientEvidenceNeverHighConfidenceApproval(t *testing.T) {
	assessment := recommendationBase()
	assessment.Eligibility.Status = EligibilityEligible
	assessment.Readiness = Readiness{Status: ReadinessInsufficientEvidence, Unknowns: 1}
	assessment.Evidence.Complete = false

	got := ApplyRecommendation(assessment)

	if got.Recommendation.Confidence == ConfidenceHigh {
		t.Fatalf("Confidence = %q, insufficient evidence must never justify high confidence", got.Recommendation.Confidence)
	}
	if got.Recommendation.Decision == RecommendationRollbackPreferred {
		t.Fatalf("Decision = %q, insufficient evidence must never justify rollback_preferred", got.Recommendation.Decision)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

// TestApplyRecommendationRollbackPreferredOnlyWhenReadinessReady is the
// structural half of the safety proof: it exhaustively checks every
// ReadinessStatus value (with eligibility held eligible so the earlier
// rollback-cannot-be-preferred guard never interferes) and confirms
// rollback_preferred is reachable if and only if Readiness.Status is ready.
// Readiness.Status is a single enum-typed field
// (internal/rollback/model.go's ReadinessStatus), so ready and
// insufficient_evidence are mutually exclusive by construction on the same
// assessment -- this test confirms ApplyRecommendation's switch respects
// that rather than merely asserting it.
func TestApplyRecommendationRollbackPreferredOnlyWhenReadinessReady(t *testing.T) {
	statuses := []ReadinessStatus{ReadinessReady, ReadinessBlocked, ReadinessHighRisk, ReadinessInsufficientEvidence}
	for _, status := range statuses {
		assessment := recommendationBase()
		assessment.Eligibility.Status = EligibilityEligible
		assessment.Readiness = Readiness{Status: status}
		assessment.Evidence.Complete = true

		got := ApplyRecommendation(assessment)
		isRollbackPreferred := got.Recommendation.Decision == RecommendationRollbackPreferred
		wantRollbackPreferred := status == ReadinessReady
		if isRollbackPreferred != wantRollbackPreferred {
			t.Fatalf("readiness %q -> rollback_preferred=%v, want %v", status, isRollbackPreferred, wantRollbackPreferred)
		}
	}
}

func TestApplyRecommendationReasonOrderingIsDeterministic(t *testing.T) {
	assessment := recommendationBase()
	assessment.Eligibility.ReasonCodes = []ReasonCode{ReasonRollbackWindowNearExpiry}
	assessment.Recommendation.ReasonCodes = []ReasonCode{
		ReasonManagedAddonCompatibilityUnknown,
		ReasonRollbackWindowNearExpiry,
	}
	assessment.Readiness = Readiness{Status: ReadinessHighRisk, Warnings: 2}
	assessment.Evidence.Complete = true
	assessment.Checks = []Check{
		{ID: "managed-addons", Status: CheckWarning, ReasonCodes: []ReasonCode{ReasonManagedAddonCompatibilityUnknown}},
		{ID: "workload-health", Status: CheckWarning, ReasonCodes: []ReasonCode{ReasonUnhealthyWorkloadsPresent}},
	}

	got := ApplyRecommendation(assessment)
	want := []ReasonCode{
		ReasonRollbackWindowNearExpiry,
		ReasonManagedAddonCompatibilityUnknown,
		ReasonUnhealthyWorkloadsPresent,
	}

	if !reflect.DeepEqual(got.Recommendation.ReasonCodes, want) {
		t.Fatalf("ReasonCodes = %v, want %v", got.Recommendation.ReasonCodes, want)
	}
}

func recommendationBase() Assessment {
	now := time.Date(2026, 7, 15, 8, 4, 0, 0, time.UTC)
	assessment := NewAssessment(ModePostUpgradeReadiness, now)
	assessment.Cluster = Cluster{
		Name:                  "prod",
		Region:                "ap-south-1",
		Provider:              "eks",
		CurrentVersion:        "1.35",
		RollbackTargetVersion: "1.34",
	}
	assessment.Eligibility = Eligibility{
		Status: EligibilityEligible,
		Source: "amazon-eks",
	}
	assessment.Readiness = Readiness{Status: ReadinessReady}
	assessment.Recommendation = Recommendation{
		Decision:   RecommendationOperatorDecisionRequired,
		Confidence: ConfidenceMedium,
	}
	assessment.Evidence = Evidence{Complete: true}
	return assessment
}

func hasRecommendationReason(assessment Assessment, want ReasonCode) bool {
	for _, got := range assessment.Recommendation.ReasonCodes {
		if got == want {
			return true
		}
	}
	return false
}
