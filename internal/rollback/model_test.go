package rollback

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAssessmentValidateAcceptsLayeredRollbackDecision(t *testing.T) {
	now := time.Date(2026, 7, 15, 8, 4, 0, 0, time.UTC)
	expires := now.Add(96 * time.Hour)
	remaining := 5760

	assessment := NewAssessment(ModePostUpgradeReadiness, now)
	assessment.Cluster = Cluster{
		Name:                  "prod",
		Region:                "ap-south-1",
		Provider:              "eks",
		CurrentVersion:        "1.35",
		RollbackTargetVersion: "1.34",
	}
	assessment.Eligibility = Eligibility{
		Status:           EligibilityEligible,
		WindowExpiresAt:  &expires,
		RemainingMinutes: &remaining,
		Source:           "amazon-eks",
	}
	assessment.Readiness = Readiness{
		Status:   ReadinessHighRisk,
		Blockers: 0,
		Warnings: 3,
		Unknowns: 1,
	}
	assessment.Recommendation = Recommendation{
		Decision:   RecommendationFixForwardPreferred,
		Confidence: ConfidenceMedium,
		ReasonCodes: []ReasonCode{
			ReasonSelfManagedAddonCompatibilityUnknown,
			ReasonUnhealthyWorkloadsPresent,
		},
	}
	assessment.Evidence = Evidence{
		ClusterObservedAt: &now,
		Complete:          false,
	}
	assessment.Checks = []Check{
		{
			ID:     "self-managed-addons",
			Title:  "Self-managed add-on rollback compatibility",
			Status: CheckUnknown,
			ReasonCodes: []ReasonCode{
				ReasonSelfManagedAddonCompatibilityUnknown,
			},
		},
	}

	if err := assessment.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	encoded, err := json.Marshal(assessment)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	for _, want := range []string{
		`"schemaVersion":"kubepreflight.io/rollback-assessment/v1alpha1"`,
		`"mode":"post_upgrade_readiness"`,
		`"status":"eligible"`,
		`"status":"high_risk"`,
		`"decision":"fix_forward_preferred"`,
		`"confidence":"medium"`,
		`"SELF_MANAGED_ADDON_COMPATIBILITY_UNKNOWN"`,
	} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("encoded assessment missing %s:\n%s", want, encoded)
		}
	}
}

func TestAssessmentValidateRejectsInvalidEnumsAndReasonCodes(t *testing.T) {
	base := validAssessment()

	tests := []struct {
		name   string
		mutate func(*Assessment)
		want   string
	}{
		{
			name: "schema",
			mutate: func(a *Assessment) {
				a.SchemaVersion = "rollback.kubepreflight.io/v0"
			},
			want: "unsupported rollback assessment schemaVersion",
		},
		{
			name: "mode",
			mutate: func(a *Assessment) {
				a.Mode = "rollback_now"
			},
			want: "unsupported rollback assessment mode",
		},
		{
			name: "eligibility",
			mutate: func(a *Assessment) {
				a.Eligibility.Status = "safe"
			},
			want: "unsupported rollback eligibility status",
		},
		{
			name: "readiness",
			mutate: func(a *Assessment) {
				a.Readiness.Status = "safe"
			},
			want: "unsupported rollback readiness status",
		},
		{
			name: "recommendation",
			mutate: func(a *Assessment) {
				a.Recommendation.Decision = "force_rollback"
			},
			want: "unsupported rollback recommendation decision",
		},
		{
			name: "confidence",
			mutate: func(a *Assessment) {
				a.Recommendation.Confidence = "certain"
			},
			want: "unsupported rollback recommendation confidence",
		},
		{
			name: "reason code",
			mutate: func(a *Assessment) {
				a.Recommendation.ReasonCodes = []ReasonCode{"ROLLBACK_IS_SAFE"}
			},
			want: "unsupported reason code",
		},
		{
			name: "check status",
			mutate: func(a *Assessment) {
				a.Checks = []Check{{ID: "x", Status: "maybe"}}
			},
			want: `check "x" has unsupported status`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assessment := base
			tc.mutate(&assessment)
			err := assessment.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestAssessmentValidateRejectsContradictoryRecommendation(t *testing.T) {
	assessment := validAssessment()
	assessment.Eligibility.Status = EligibilityUnavailable
	assessment.Recommendation.Decision = RecommendationRollbackPreferred

	err := assessment.Validate()
	if err == nil || !strings.Contains(err.Error(), "rollback cannot be preferred") {
		t.Fatalf("Validate() error = %v, want rollback cannot be preferred", err)
	}
}

func TestAssessmentValidateRejectsHighConfidenceWithInsufficientEvidence(t *testing.T) {
	assessment := validAssessment()
	assessment.Readiness.Status = ReadinessInsufficientEvidence
	assessment.Recommendation.Confidence = ConfidenceHigh

	err := assessment.Validate()
	if err == nil || !strings.Contains(err.Error(), "high-confidence recommendation") {
		t.Fatalf("Validate() error = %v, want high-confidence recommendation guard", err)
	}
}

func validAssessment() Assessment {
	now := time.Date(2026, 7, 15, 8, 4, 0, 0, time.UTC)
	assessment := NewAssessment(ModePreUpgradePosture, now)
	assessment.Cluster = Cluster{
		Name:                  "prod",
		Region:                "ap-south-1",
		Provider:              "eks",
		CurrentVersion:        "1.34",
		RollbackTargetVersion: "1.33",
	}
	assessment.Eligibility = Eligibility{
		Status:      EligibilityEligible,
		Source:      "amazon-eks",
		ReasonCodes: []ReasonCode{ReasonRollbackWindowNearExpiry},
	}
	assessment.Readiness = Readiness{
		Status: ReadinessHighRisk,
	}
	assessment.Recommendation = Recommendation{
		Decision:    RecommendationFixForwardPreferred,
		Confidence:  ConfidenceMedium,
		ReasonCodes: []ReasonCode{ReasonRollbackWindowNearExpiry},
	}
	assessment.Evidence = Evidence{Complete: true}
	assessment.Checks = []Check{
		{ID: "rollback-window", Title: "Rollback window", Status: CheckWarning, ReasonCodes: []ReasonCode{ReasonRollbackWindowNearExpiry}},
	}
	return assessment
}
