// Package rollback defines the assessment document used by the EKS
// rollback-readiness workflow. It is intentionally model-only for now:
// collectors, rules, and CLI presentation are added in later slices.
package rollback

import (
	"fmt"
	"time"
)

const SchemaVersion = "kubepreflight.io/rollback-assessment/v1alpha1"

type AssessmentMode string

const (
	ModePreUpgradePosture    AssessmentMode = "pre_upgrade_posture"
	ModePostUpgradeReadiness AssessmentMode = "post_upgrade_readiness"
)

type EligibilityStatus string

const (
	EligibilityEligible    EligibilityStatus = "eligible"
	EligibilityUnavailable EligibilityStatus = "unavailable"
	EligibilityUnknown     EligibilityStatus = "unknown"
)

type ReadinessStatus string

const (
	ReadinessReady                ReadinessStatus = "ready"
	ReadinessBlocked              ReadinessStatus = "blocked"
	ReadinessHighRisk             ReadinessStatus = "high_risk"
	ReadinessInsufficientEvidence ReadinessStatus = "insufficient_evidence"
)

type RecommendationDecision string

const (
	RecommendationRollbackPreferred        RecommendationDecision = "rollback_preferred"
	RecommendationFixForwardPreferred      RecommendationDecision = "fix_forward_preferred"
	RecommendationOperatorDecisionRequired RecommendationDecision = "operator_decision_required"
	RecommendationDoNotProceed             RecommendationDecision = "do_not_proceed"
)

type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

type ReasonCode string

const (
	ReasonUpgradeHistoryUnavailable              ReasonCode = "UPGRADE_HISTORY_UNAVAILABLE"
	ReasonEKSUpgradeHistoryUnavailable           ReasonCode = "EKS_UPGRADE_HISTORY_UNAVAILABLE"
	ReasonUpgradeWasNotInPlace                   ReasonCode = "UPGRADE_WAS_NOT_IN_PLACE"
	ReasonRollbackWindowExpired                  ReasonCode = "ROLLBACK_WINDOW_EXPIRED"
	ReasonRollbackWindowNearExpiry               ReasonCode = "ROLLBACK_WINDOW_NEAR_EXPIRY"
	ReasonPreviousVersionNotNMinusOne            ReasonCode = "PREVIOUS_VERSION_NOT_N_MINUS_ONE"
	ReasonClusterNotActive                       ReasonCode = "CLUSTER_NOT_ACTIVE"
	ReasonRollbackTargetUnsupported              ReasonCode = "ROLLBACK_TARGET_UNSUPPORTED"
	ReasonRollbackTargetRequiresExtendedSupport  ReasonCode = "ROLLBACK_TARGET_REQUIRES_EXTENDED_SUPPORT"
	ReasonUpgradePolicyDisallowsRollbackTarget   ReasonCode = "UPGRADE_POLICY_DISALLOWS_ROLLBACK_TARGET"
	ReasonEndOfExtendedSupportAutoUpgrade        ReasonCode = "END_OF_EXTENDED_SUPPORT_AUTO_UPGRADE"
	ReasonEndOfExtendedSupportAutoUpgradeUnknown ReasonCode = "END_OF_EXTENDED_SUPPORT_AUTO_UPGRADE_UNVERIFIED"
	ReasonEKSFeatureCompatibilityUnverified      ReasonCode = "EKS_FEATURE_COMPATIBILITY_UNVERIFIED"
	ReasonEKSFeatureIncompatible                 ReasonCode = "EKS_FEATURE_INCOMPATIBLE"
	ReasonIncompatibleEKSFeatureEnabled          ReasonCode = "INCOMPATIBLE_EKS_FEATURE_ENABLED"
	ReasonEKSInsightsUnavailable                 ReasonCode = "EKS_INSIGHTS_UNAVAILABLE"
	ReasonEKSInsightsStale                       ReasonCode = "EKS_INSIGHTS_STALE"
	ReasonEKSInsightsBlocking                    ReasonCode = "EKS_INSIGHTS_BLOCKING"
	ReasonManagedNodegroupRollbackRequired       ReasonCode = "MANAGED_NODEGROUP_ROLLBACK_REQUIRED"
	ReasonSelfManagedNodeEvidenceUnavailable     ReasonCode = "SELF_MANAGED_NODE_EVIDENCE_UNAVAILABLE"
	ReasonSelfManagedNodeRollbackRequired        ReasonCode = "SELF_MANAGED_NODE_ROLLBACK_REQUIRED"
	ReasonWorkerNodesWouldRemainNewerThanControl ReasonCode = "WORKER_NODES_WOULD_REMAIN_NEWER_THAN_CONTROL_PLANE"
	ReasonAutoModeDisruptionRisk                 ReasonCode = "AUTO_MODE_DISRUPTION_RISK"
	ReasonFargateEvidenceUnavailable             ReasonCode = "FARGATE_EVIDENCE_UNAVAILABLE"
	ReasonFargatePodRecreationRisk               ReasonCode = "FARGATE_POD_RECREATION_RISK"
	ReasonManagedAddonRollbackRequired           ReasonCode = "MANAGED_ADDON_ROLLBACK_REQUIRED"
	ReasonManagedAddonCompatibilityUnknown       ReasonCode = "MANAGED_ADDON_COMPATIBILITY_UNKNOWN"
	ReasonSelfManagedAddonCompatibilityUnknown   ReasonCode = "SELF_MANAGED_ADDON_COMPATIBILITY_UNKNOWN"
	ReasonNewVersionAPIAdoptionRisk              ReasonCode = "NEW_VERSION_API_ADOPTION_RISK"
	ReasonCRDWebhookControllerRisk               ReasonCode = "CRD_WEBHOOK_CONTROLLER_RISK"
	ReasonPDBDisruptionConstraints               ReasonCode = "PDB_DISRUPTION_CONSTRAINTS"
	ReasonUnhealthyWorkloadsPresent              ReasonCode = "UNHEALTHY_WORKLOADS_PRESENT"
	ReasonObservabilityEvidenceMissing           ReasonCode = "OBSERVABILITY_EVIDENCE_MISSING"
	ReasonApplicationHealthUnknown               ReasonCode = "APPLICATION_HEALTH_UNKNOWN"
	ReasonForceBypassNotRecommended              ReasonCode = "FORCE_BYPASS_NOT_RECOMMENDED"
)

// Assessment is the top-level rollback assessment JSON document.
type Assessment struct {
	SchemaVersion  string         `json:"schemaVersion"`
	Mode           AssessmentMode `json:"mode"`
	Cluster        Cluster        `json:"cluster"`
	Eligibility    Eligibility    `json:"eligibility"`
	Readiness      Readiness      `json:"readiness"`
	Recommendation Recommendation `json:"recommendation"`
	Evidence       Evidence       `json:"evidence"`
	Checks         []Check        `json:"checks,omitempty"`
	GeneratedAt    time.Time      `json:"generatedAt"`
}

type Cluster struct {
	Name                  string `json:"name"`
	Region                string `json:"region,omitempty"`
	CurrentVersion        string `json:"currentVersion"`
	RollbackTargetVersion string `json:"rollbackTargetVersion,omitempty"`
	Provider              string `json:"provider,omitempty"`
}

type Eligibility struct {
	Status           EligibilityStatus `json:"status"`
	WindowExpiresAt  *time.Time        `json:"windowExpiresAt,omitempty"`
	RemainingMinutes *int              `json:"remainingMinutes,omitempty"`
	Source           string            `json:"source,omitempty"`
	ReasonCodes      []ReasonCode      `json:"reasonCodes,omitempty"`
}

type Readiness struct {
	Status   ReadinessStatus `json:"status"`
	Blockers int             `json:"blockers"`
	Warnings int             `json:"warnings"`
	Unknowns int             `json:"unknowns"`
}

type Recommendation struct {
	Decision    RecommendationDecision `json:"decision"`
	Confidence  Confidence             `json:"confidence"`
	ReasonCodes []ReasonCode           `json:"reasonCodes,omitempty"`
}

type Evidence struct {
	EKSInsightsRefreshedAt *time.Time `json:"eksInsightsRefreshedAt,omitempty"`
	ClusterObservedAt      *time.Time `json:"clusterObservedAt,omitempty"`
	Complete               bool       `json:"complete"`
	WindowCalculation      string     `json:"windowCalculation,omitempty"`
	TimestampSource        string     `json:"timestampSource,omitempty"`
}

type CheckStatus string

const (
	CheckPass    CheckStatus = "pass"
	CheckWarning CheckStatus = "warning"
	CheckFail    CheckStatus = "fail"
	CheckUnknown CheckStatus = "unknown"
)

type Check struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Status      CheckStatus  `json:"status"`
	ReasonCodes []ReasonCode `json:"reasonCodes,omitempty"`
	Evidence    []string     `json:"evidence,omitempty"`
}

func NewAssessment(mode AssessmentMode, now time.Time) Assessment {
	return Assessment{
		SchemaVersion: SchemaVersion,
		Mode:          mode,
		GeneratedAt:   now,
	}
}

func (a Assessment) Validate() error {
	if a.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported rollback assessment schemaVersion %q", a.SchemaVersion)
	}
	if !validAssessmentMode(a.Mode) {
		return fmt.Errorf("unsupported rollback assessment mode %q", a.Mode)
	}
	if !validEligibilityStatus(a.Eligibility.Status) {
		return fmt.Errorf("unsupported rollback eligibility status %q", a.Eligibility.Status)
	}
	if !validReadinessStatus(a.Readiness.Status) {
		return fmt.Errorf("unsupported rollback readiness status %q", a.Readiness.Status)
	}
	if !validRecommendationDecision(a.Recommendation.Decision) {
		return fmt.Errorf("unsupported rollback recommendation decision %q", a.Recommendation.Decision)
	}
	if !validConfidence(a.Recommendation.Confidence) {
		return fmt.Errorf("unsupported rollback recommendation confidence %q", a.Recommendation.Confidence)
	}
	if err := validateReasonCodes(a.Eligibility.ReasonCodes); err != nil {
		return fmt.Errorf("eligibility: %w", err)
	}
	if err := validateReasonCodes(a.Recommendation.ReasonCodes); err != nil {
		return fmt.Errorf("recommendation: %w", err)
	}
	for _, check := range a.Checks {
		if !validCheckStatus(check.Status) {
			return fmt.Errorf("check %q has unsupported status %q", check.ID, check.Status)
		}
		if err := validateReasonCodes(check.ReasonCodes); err != nil {
			return fmt.Errorf("check %q: %w", check.ID, err)
		}
	}
	if a.Eligibility.Status != EligibilityEligible && a.Recommendation.Decision == RecommendationRollbackPreferred {
		return fmt.Errorf("rollback cannot be preferred when eligibility is %q", a.Eligibility.Status)
	}
	if a.Readiness.Status == ReadinessInsufficientEvidence && a.Recommendation.Confidence == ConfidenceHigh {
		return fmt.Errorf("high-confidence recommendation requires sufficient rollback evidence")
	}
	return nil
}

func validateReasonCodes(codes []ReasonCode) error {
	for _, code := range codes {
		if !validReasonCode(code) {
			return fmt.Errorf("unsupported reason code %q", code)
		}
	}
	return nil
}

func validAssessmentMode(mode AssessmentMode) bool {
	return mode == ModePreUpgradePosture || mode == ModePostUpgradeReadiness
}

func validEligibilityStatus(status EligibilityStatus) bool {
	return status == EligibilityEligible || status == EligibilityUnavailable || status == EligibilityUnknown
}

func validReadinessStatus(status ReadinessStatus) bool {
	return status == ReadinessReady || status == ReadinessBlocked ||
		status == ReadinessHighRisk || status == ReadinessInsufficientEvidence
}

func validRecommendationDecision(decision RecommendationDecision) bool {
	return decision == RecommendationRollbackPreferred || decision == RecommendationFixForwardPreferred ||
		decision == RecommendationOperatorDecisionRequired || decision == RecommendationDoNotProceed
}

func validConfidence(confidence Confidence) bool {
	return confidence == ConfidenceHigh || confidence == ConfidenceMedium || confidence == ConfidenceLow
}

func validCheckStatus(status CheckStatus) bool {
	return status == CheckPass || status == CheckWarning || status == CheckFail || status == CheckUnknown
}

func validReasonCode(code ReasonCode) bool {
	switch code {
	case ReasonUpgradeHistoryUnavailable,
		ReasonEKSUpgradeHistoryUnavailable,
		ReasonUpgradeWasNotInPlace,
		ReasonRollbackWindowExpired,
		ReasonRollbackWindowNearExpiry,
		ReasonPreviousVersionNotNMinusOne,
		ReasonClusterNotActive,
		ReasonRollbackTargetUnsupported,
		ReasonRollbackTargetRequiresExtendedSupport,
		ReasonUpgradePolicyDisallowsRollbackTarget,
		ReasonEndOfExtendedSupportAutoUpgrade,
		ReasonEndOfExtendedSupportAutoUpgradeUnknown,
		ReasonEKSFeatureCompatibilityUnverified,
		ReasonEKSFeatureIncompatible,
		ReasonIncompatibleEKSFeatureEnabled,
		ReasonEKSInsightsUnavailable,
		ReasonEKSInsightsStale,
		ReasonEKSInsightsBlocking,
		ReasonManagedNodegroupRollbackRequired,
		ReasonSelfManagedNodeEvidenceUnavailable,
		ReasonSelfManagedNodeRollbackRequired,
		ReasonWorkerNodesWouldRemainNewerThanControl,
		ReasonAutoModeDisruptionRisk,
		ReasonFargateEvidenceUnavailable,
		ReasonFargatePodRecreationRisk,
		ReasonManagedAddonRollbackRequired,
		ReasonManagedAddonCompatibilityUnknown,
		ReasonSelfManagedAddonCompatibilityUnknown,
		ReasonNewVersionAPIAdoptionRisk,
		ReasonCRDWebhookControllerRisk,
		ReasonPDBDisruptionConstraints,
		ReasonUnhealthyWorkloadsPresent,
		ReasonObservabilityEvidenceMissing,
		ReasonApplicationHealthUnknown,
		ReasonForceBypassNotRecommended:
		return true
	default:
		return false
	}
}
