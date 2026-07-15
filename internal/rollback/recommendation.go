package rollback

// ApplyRecommendation collapses eligibility, readiness, and evidence
// completeness into the final rollback-vs-fix-forward decision.
func ApplyRecommendation(assessment Assessment) Assessment {
	reasons := recommendationReasons(assessment)

	switch {
	case assessment.Eligibility.Status == EligibilityUnavailable:
		assessment.Recommendation = Recommendation{
			Decision:    RecommendationDoNotProceed,
			Confidence:  ConfidenceHigh,
			ReasonCodes: reasons,
		}
	case assessment.Readiness.Status == ReadinessBlocked:
		assessment.Recommendation = Recommendation{
			Decision:    RecommendationDoNotProceed,
			Confidence:  ConfidenceHigh,
			ReasonCodes: reasons,
		}
	case assessment.Eligibility.Status == EligibilityUnknown:
		assessment.Recommendation = Recommendation{
			Decision:    RecommendationOperatorDecisionRequired,
			Confidence:  ConfidenceLow,
			ReasonCodes: reasons,
		}
	case assessment.Readiness.Status == ReadinessInsufficientEvidence || !assessment.Evidence.Complete:
		assessment.Recommendation = Recommendation{
			Decision:    RecommendationOperatorDecisionRequired,
			Confidence:  ConfidenceLow,
			ReasonCodes: reasons,
		}
	case assessment.Readiness.Status == ReadinessHighRisk:
		assessment.Recommendation = Recommendation{
			Decision:    RecommendationFixForwardPreferred,
			Confidence:  ConfidenceMedium,
			ReasonCodes: reasons,
		}
	case assessment.Readiness.Status == ReadinessReady:
		assessment.Recommendation = Recommendation{
			Decision:    RecommendationRollbackPreferred,
			Confidence:  ConfidenceMedium,
			ReasonCodes: reasons,
		}
	}

	return assessment
}

func recommendationReasons(assessment Assessment) []ReasonCode {
	var reasons []ReasonCode
	for _, reason := range assessment.Eligibility.ReasonCodes {
		reasons = appendUniqueReason(reasons, reason)
	}
	for _, reason := range assessment.Recommendation.ReasonCodes {
		reasons = appendUniqueReason(reasons, reason)
	}
	for _, check := range assessment.Checks {
		for _, reason := range check.ReasonCodes {
			reasons = appendUniqueReason(reasons, reason)
		}
	}
	return reasons
}
