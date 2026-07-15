package eks

import (
	"fmt"
	"time"

	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"kubepreflight/internal/rollback"
)

const insightFreshnessWindow = 24 * time.Hour

func ApplyRollbackInsights(assessment rollback.Assessment, snap *Snapshot, now time.Time) rollback.Assessment {
	if snap == nil || hasErrors(snap, "list-rollback-insights") || hasErrorPrefix(snap, "describe-rollback-insight:") {
		return markInsightsUnavailable(assessment)
	}

	assessment.Recommendation.ReasonCodes = removeReasonCode(assessment.Recommendation.ReasonCodes, rollback.ReasonEKSFeatureCompatibilityUnverified)

	var blockers, warnings, stale int
	var insightChecks []rollback.Check
	var oldestRefresh *time.Time
	for _, insight := range snap.Insights {
		if !insight.LastRefreshTime.IsZero() {
			refresh := insight.LastRefreshTime
			if oldestRefresh == nil || refresh.Before(*oldestRefresh) {
				oldestRefresh = &refresh
			}
			if now.Sub(refresh) > insightFreshnessWindow {
				stale++
			}
		} else {
			stale++
		}

		switch insight.Status {
		case string(ekstypes.InsightStatusValueError), string(ekstypes.InsightStatusValueUnknown):
			blockers++
		case string(ekstypes.InsightStatusValueWarning):
			warnings++
		}
		insightChecks = append(insightChecks, insightCheck(insight, now))
	}
	if oldestRefresh != nil {
		assessment.Evidence.EKSInsightsRefreshedAt = oldestRefresh
	}

	assessment.Checks = append(assessment.Checks, insightChecks...)

	if stale > 0 {
		return markInsightsStale(assessment, stale)
	}
	if blockers > 0 {
		assessment.Readiness = rollback.Readiness{Status: rollback.ReadinessBlocked, Blockers: blockers, Warnings: warnings}
		assessment.Recommendation.Decision = rollback.RecommendationDoNotProceed
		assessment.Recommendation.Confidence = rollback.ConfidenceHigh
		assessment.Recommendation.ReasonCodes = uniqueReasonCodes(append(assessment.Recommendation.ReasonCodes, rollback.ReasonEKSInsightsBlocking))
		assessment.Evidence.Complete = false
		return assessment
	}
	if warnings > 0 {
		assessment.Readiness = rollback.Readiness{Status: rollback.ReadinessHighRisk, Warnings: warnings}
		assessment.Recommendation.Decision = rollback.RecommendationOperatorDecisionRequired
		assessment.Recommendation.Confidence = rollback.ConfidenceMedium
		assessment.Evidence.Complete = remainingUnknownEvidence(assessment) == 0
		return assessment
	}

	if remainingUnknownEvidence(assessment) > 0 {
		assessment.Readiness = rollback.Readiness{Status: rollback.ReadinessInsufficientEvidence, Unknowns: remainingUnknownEvidence(assessment)}
		assessment.Recommendation.Confidence = rollback.ConfidenceLow
		assessment.Evidence.Complete = false
		return assessment
	}

	assessment.Readiness = rollback.Readiness{Status: rollback.ReadinessReady}
	assessment.Recommendation.Decision = rollback.RecommendationOperatorDecisionRequired
	assessment.Recommendation.Confidence = rollback.ConfidenceMedium
	assessment.Evidence.Complete = true
	return assessment
}

func markInsightsUnavailable(assessment rollback.Assessment) rollback.Assessment {
	assessment.Readiness = rollback.Readiness{Status: rollback.ReadinessInsufficientEvidence, Unknowns: 1}
	assessment.Recommendation.Decision = rollback.RecommendationOperatorDecisionRequired
	assessment.Recommendation.Confidence = rollback.ConfidenceLow
	assessment.Recommendation.ReasonCodes = uniqueReasonCodes(append(assessment.Recommendation.ReasonCodes, rollback.ReasonEKSInsightsUnavailable))
	assessment.Evidence.Complete = false
	assessment.Checks = append(assessment.Checks, rollback.Check{
		ID:          "rollback-insights",
		Title:       "EKS rollback readiness insights are available",
		Status:      rollback.CheckUnknown,
		ReasonCodes: []rollback.ReasonCode{rollback.ReasonEKSInsightsUnavailable},
	})
	return assessment
}

func markInsightsStale(assessment rollback.Assessment, count int) rollback.Assessment {
	assessment.Readiness = rollback.Readiness{Status: rollback.ReadinessInsufficientEvidence, Unknowns: count}
	assessment.Recommendation.Decision = rollback.RecommendationOperatorDecisionRequired
	assessment.Recommendation.Confidence = rollback.ConfidenceLow
	assessment.Recommendation.ReasonCodes = uniqueReasonCodes(append(assessment.Recommendation.ReasonCodes, rollback.ReasonEKSInsightsStale))
	assessment.Evidence.Complete = false
	return assessment
}

func insightCheck(insight InsightRecord, now time.Time) rollback.Check {
	check := rollback.Check{
		ID:     "rollback-insight-" + insight.ID,
		Title:  emptyAsUnknown(insight.Name),
		Status: insightCheckStatus(insight.Status),
		Evidence: []string{
			"insight id: " + emptyAsUnknown(insight.ID),
			"status: " + emptyAsUnknown(insight.Status),
			"description: " + emptyAsUnknown(insight.Description),
		},
	}
	if !insight.LastRefreshTime.IsZero() {
		check.Evidence = append(check.Evidence, "lastRefreshTime: "+insight.LastRefreshTime.Format(time.RFC3339))
		if now.Sub(insight.LastRefreshTime) > insightFreshnessWindow {
			check.Status = rollback.CheckUnknown
			check.ReasonCodes = append(check.ReasonCodes, rollback.ReasonEKSInsightsStale)
		}
	} else {
		check.Status = rollback.CheckUnknown
		check.ReasonCodes = append(check.ReasonCodes, rollback.ReasonEKSInsightsStale)
	}
	if !insight.LastTransitionTime.IsZero() {
		check.Evidence = append(check.Evidence, "lastTransitionTime: "+insight.LastTransitionTime.Format(time.RFC3339))
	}
	if insight.Recommendation != "" {
		check.Evidence = append(check.Evidence, "recommendation: "+insight.Recommendation)
	}
	for _, resource := range insight.Resources {
		check.Evidence = append(check.Evidence, fmt.Sprintf("resource: arn=%s uri=%s status=%s reason=%s",
			emptyAsUnknown(resource.ARN),
			emptyAsUnknown(resource.KubernetesResourceURI),
			emptyAsUnknown(resource.Status),
			emptyAsUnknown(resource.Reason),
		))
	}
	switch insight.Status {
	case string(ekstypes.InsightStatusValueError), string(ekstypes.InsightStatusValueUnknown):
		check.ReasonCodes = uniqueReasonCodes(append(check.ReasonCodes, rollback.ReasonEKSInsightsBlocking))
	}
	return check
}

func insightCheckStatus(status string) rollback.CheckStatus {
	switch status {
	case string(ekstypes.InsightStatusValuePassing):
		return rollback.CheckPass
	case string(ekstypes.InsightStatusValueWarning):
		return rollback.CheckWarning
	case string(ekstypes.InsightStatusValueError):
		return rollback.CheckFail
	default:
		return rollback.CheckUnknown
	}
}

func removeReasonCode(codes []rollback.ReasonCode, remove rollback.ReasonCode) []rollback.ReasonCode {
	out := codes[:0]
	for _, code := range codes {
		if code != remove {
			out = append(out, code)
		}
	}
	return out
}

func remainingUnknownEvidence(assessment rollback.Assessment) int {
	count := 0
	for _, code := range assessment.Recommendation.ReasonCodes {
		switch code {
		case rollback.ReasonEndOfExtendedSupportAutoUpgradeUnknown,
			rollback.ReasonEKSFeatureCompatibilityUnverified,
			rollback.ReasonEKSInsightsUnavailable,
			rollback.ReasonEKSInsightsStale:
			count++
		}
	}
	return count
}
