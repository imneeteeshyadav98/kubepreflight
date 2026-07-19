package eks

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/imneeteeshyadav98/kubepreflight/internal/rollback"
)

const rollbackWindow = 7 * 24 * time.Hour

func EvaluateEligibility(snap *Snapshot, now time.Time) rollback.Assessment {
	assessment := rollback.NewAssessment(rollback.ModePostUpgradeReadiness, now)
	assessment.Evidence.ClusterObservedAt = &now

	if snap == nil {
		return unknownAssessment(assessment, rollback.ReasonEKSUpgradeHistoryUnavailable)
	}

	assessment.Cluster = rollback.Cluster{
		Name:                  snap.ClusterName,
		Region:                snap.Region,
		Provider:              "eks",
		CurrentVersion:        snap.CurrentVersion,
		RollbackTargetVersion: previousMinor(snap.CurrentVersion),
	}
	if !snap.ObservedAt.IsZero() {
		assessment.Evidence.ClusterObservedAt = &snap.ObservedAt
	}

	if hasErrors(snap, "describe-cluster", "list-updates", "describe-cluster-versions") || hasErrorPrefix(snap, "describe-update:") {
		return unknownAssessment(assessment, rollback.ReasonEKSUpgradeHistoryUnavailable)
	}

	var reasons []rollback.ReasonCode
	if snap.ClusterStatus != "" && snap.ClusterStatus != "ACTIVE" {
		reasons = append(reasons, rollback.ReasonClusterNotActive)
	}

	target := assessment.Cluster.RollbackTargetVersion
	targetRecord, targetFound := targetVersionRecord(snap.ClusterVersions, target)
	if target == "" {
		reasons = append(reasons, rollback.ReasonPreviousVersionNotNMinusOne)
	} else if !targetFound || !isSupportedTargetStatus(targetRecord.Status) {
		reasons = append(reasons, rollback.ReasonRollbackTargetUnsupported)
	} else if targetRecord.Status == string(ekstypes.VersionStatusExtendedSupport) && snap.SupportType != string(ekstypes.SupportTypeExtended) {
		reasons = append(reasons, rollback.ReasonRollbackTargetRequiresExtendedSupport)
	}

	update, ok := latestSuccessfulVersionUpdate(snap.Updates, snap.CurrentVersion)
	if !ok {
		reasons = append(reasons, rollback.ReasonEKSUpgradeHistoryUnavailable)
	} else {
		expires := update.CreatedAt.Add(rollbackWindow)
		remaining := int(expires.Sub(now).Minutes())
		if remaining < 0 {
			remaining = 0
		}
		assessment.Eligibility.WindowExpiresAt = &expires
		assessment.Eligibility.RemainingMinutes = &remaining
		assessment.Evidence.WindowCalculation = "conservative"
		assessment.Evidence.TimestampSource = "eks_update_created_at"
		if !previousVersionIsNMinusOne(snap.CurrentVersion, target) {
			reasons = append(reasons, rollback.ReasonPreviousVersionNotNMinusOne)
		}
		if !now.Before(expires) {
			reasons = append(reasons, rollback.ReasonRollbackWindowExpired)
		}
	}

	assessment.Eligibility.Source = "amazon-eks"
	assessment.Eligibility.ReasonCodes = uniqueReasonCodes(reasons)
	assessment.Evidence.Complete = len(reasons) == 0
	readinessUnknownReasons := []rollback.ReasonCode{
		rollback.ReasonEndOfExtendedSupportAutoUpgradeUnknown,
		rollback.ReasonEKSFeatureCompatibilityUnverified,
	}

	if len(reasons) > 0 {
		assessment.Eligibility.Status = rollback.EligibilityUnavailable
		assessment.Readiness = rollback.Readiness{Status: rollback.ReadinessBlocked, Blockers: len(assessment.Eligibility.ReasonCodes)}
		assessment.Recommendation = rollback.Recommendation{
			Decision:    rollback.RecommendationDoNotProceed,
			Confidence:  rollback.ConfidenceHigh,
			ReasonCodes: assessment.Eligibility.ReasonCodes,
		}
	} else {
		assessment.Eligibility.Status = rollback.EligibilityEligible
		assessment.Readiness = rollback.Readiness{Status: rollback.ReadinessInsufficientEvidence, Unknowns: len(readinessUnknownReasons)}
		assessment.Recommendation = rollback.Recommendation{
			Decision:    rollback.RecommendationOperatorDecisionRequired,
			Confidence:  rollback.ConfidenceLow,
			ReasonCodes: readinessUnknownReasons,
		}
		assessment.Evidence.Complete = false
	}

	assessment.Checks = eligibilityChecks(snap, assessment, update, ok)
	return assessment
}

func unknownAssessment(assessment rollback.Assessment, reason rollback.ReasonCode) rollback.Assessment {
	assessment.Eligibility = rollback.Eligibility{
		Status:      rollback.EligibilityUnknown,
		Source:      "amazon-eks",
		ReasonCodes: []rollback.ReasonCode{reason},
	}
	assessment.Readiness = rollback.Readiness{
		Status:   rollback.ReadinessInsufficientEvidence,
		Unknowns: 1,
	}
	assessment.Recommendation = rollback.Recommendation{
		Decision:    rollback.RecommendationOperatorDecisionRequired,
		Confidence:  rollback.ConfidenceLow,
		ReasonCodes: []rollback.ReasonCode{reason},
	}
	assessment.Evidence.Complete = false
	return assessment
}

func hasErrors(snap *Snapshot, keys ...string) bool {
	for _, key := range keys {
		if _, ok := snap.Errors[key]; ok {
			return true
		}
	}
	return false
}

func hasErrorPrefix(snap *Snapshot, prefix string) bool {
	for key := range snap.Errors {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

func latestSuccessfulVersionUpdate(updates []UpdateRecord, currentVersion string) (UpdateRecord, bool) {
	var matches []UpdateRecord
	for _, update := range updates {
		if update.Type != string(ekstypes.UpdateTypeVersionUpdate) || update.Status != string(ekstypes.UpdateStatusSuccessful) {
			continue
		}
		if currentVersion != "" && update.Version != "" && update.Version != currentVersion {
			continue
		}
		matches = append(matches, update)
	}
	if len(matches) == 0 {
		return UpdateRecord{}, false
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].CreatedAt.After(matches[j].CreatedAt)
	})
	return matches[0], true
}

func previousMinor(version string) string {
	major, minor, ok := parseMinor(version)
	if !ok || minor == 0 {
		return ""
	}
	return fmt.Sprintf("%d.%d", major, minor-1)
}

func previousVersionIsNMinusOne(currentVersion, previousVersion string) bool {
	return previousVersion != "" && previousMinor(currentVersion) == previousVersion
}

func parseMinor(version string) (int, int, bool) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(version), "v")
	parts := strings.Split(trimmed, ".")
	if len(parts) < 2 {
		return 0, 0, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, false
	}
	return major, minor, true
}

func targetVersionRecord(versions []ClusterVersionRecord, target string) (ClusterVersionRecord, bool) {
	for _, version := range versions {
		if version.Version == target {
			return version, true
		}
	}
	return ClusterVersionRecord{}, false
}

func isSupportedTargetStatus(status string) bool {
	return status == string(ekstypes.VersionStatusStandardSupport) ||
		status == string(ekstypes.VersionStatusExtendedSupport)
}

func uniqueReasonCodes(reasons []rollback.ReasonCode) []rollback.ReasonCode {
	seen := map[rollback.ReasonCode]bool{}
	var out []rollback.ReasonCode
	for _, reason := range reasons {
		if seen[reason] {
			continue
		}
		seen[reason] = true
		out = append(out, reason)
	}
	return out
}

func eligibilityChecks(snap *Snapshot, assessment rollback.Assessment, update UpdateRecord, hasUpdate bool) []rollback.Check {
	checks := []rollback.Check{
		{
			ID:     "cluster-active",
			Title:  "EKS cluster status is ACTIVE",
			Status: statusForReason(assessment, rollback.ReasonClusterNotActive),
			Evidence: []string{
				"status: " + emptyAsUnknown(snap.ClusterStatus),
			},
			ReasonCodes: reasonIfPresent(assessment, rollback.ReasonClusterNotActive),
		},
		{
			ID:     "rollback-target-supported",
			Title:  "Rollback target EKS version is supported",
			Status: statusForReason(assessment, rollback.ReasonRollbackTargetUnsupported),
			Evidence: []string{
				"target version: " + emptyAsUnknown(assessment.Cluster.RollbackTargetVersion),
				"target versionStatus: " + emptyAsUnknown(targetStatus(snap.ClusterVersions, assessment.Cluster.RollbackTargetVersion)),
			},
			ReasonCodes: reasonIfPresent(assessment, rollback.ReasonRollbackTargetUnsupported),
		},
		{
			ID:     "extended-support-policy",
			Title:  "Cluster upgrade policy allows extended-support rollback target",
			Status: statusForReason(assessment, rollback.ReasonRollbackTargetRequiresExtendedSupport),
			Evidence: []string{
				"upgrade policy supportType: " + emptyAsUnknown(snap.SupportType),
				"target versionStatus: " + emptyAsUnknown(targetStatus(snap.ClusterVersions, assessment.Cluster.RollbackTargetVersion)),
			},
			ReasonCodes: reasonIfPresent(assessment, rollback.ReasonRollbackTargetRequiresExtendedSupport),
		},
		{
			ID:     "previous-version",
			Title:  "Previous version is exactly N-1",
			Status: statusForReason(assessment, rollback.ReasonPreviousVersionNotNMinusOne),
			Evidence: []string{
				"current version: " + emptyAsUnknown(snap.CurrentVersion),
				"rollback target version: " + emptyAsUnknown(assessment.Cluster.RollbackTargetVersion),
			},
			ReasonCodes: reasonIfPresent(assessment, rollback.ReasonPreviousVersionNotNMinusOne),
		},
	}
	windowCheck := rollback.Check{
		ID:     "rollback-window",
		Title:  "EKS rollback window is active",
		Status: statusForReason(assessment, rollback.ReasonRollbackWindowExpired),
	}
	if hasUpdate {
		windowCheck.Evidence = []string{
			"upgrade update id: " + update.ID,
			"upgrade update createdAt: " + update.CreatedAt.Format(time.RFC3339),
			"window calculation: conservative",
			"timestamp source: eks_update_created_at",
		}
	} else {
		windowCheck.Status = rollback.CheckUnknown
		windowCheck.ReasonCodes = []rollback.ReasonCode{rollback.ReasonEKSUpgradeHistoryUnavailable}
	}
	windowCheck.ReasonCodes = append(windowCheck.ReasonCodes, reasonIfPresent(assessment, rollback.ReasonRollbackWindowExpired)...)
	checks = append(checks, windowCheck)
	checks = append(checks,
		rollback.Check{
			ID:     "end-of-extended-support-auto-upgrade",
			Title:  "End-of-extended-support auto-upgrade origin is not yet verified",
			Status: rollback.CheckUnknown,
			ReasonCodes: []rollback.ReasonCode{
				rollback.ReasonEndOfExtendedSupportAutoUpgradeUnknown,
			},
		},
		rollback.Check{
			ID:     "eks-feature-compatibility",
			Title:  "Backward-incompatible EKS feature compatibility is not yet verified",
			Status: rollback.CheckUnknown,
			ReasonCodes: []rollback.ReasonCode{
				rollback.ReasonEKSFeatureCompatibilityUnverified,
			},
		},
	)
	return checks
}

func targetStatus(versions []ClusterVersionRecord, target string) string {
	rec, ok := targetVersionRecord(versions, target)
	if !ok {
		return ""
	}
	return rec.Status
}

func statusForReason(assessment rollback.Assessment, reason rollback.ReasonCode) rollback.CheckStatus {
	for _, present := range assessment.Eligibility.ReasonCodes {
		if present == reason {
			return rollback.CheckFail
		}
	}
	if assessment.Eligibility.Status == rollback.EligibilityUnknown {
		return rollback.CheckUnknown
	}
	return rollback.CheckPass
}

func reasonIfPresent(assessment rollback.Assessment, reason rollback.ReasonCode) []rollback.ReasonCode {
	for _, present := range assessment.Eligibility.ReasonCodes {
		if present == reason {
			return []rollback.ReasonCode{reason}
		}
	}
	return nil
}

func emptyAsUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}
