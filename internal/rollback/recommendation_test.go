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
