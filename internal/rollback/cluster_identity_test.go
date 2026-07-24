package rollback

import (
	"testing"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

// clusterSpecificCheckIDs are every check family this PR gates on cluster
// identity -- see validateClusterEvidenceIdentity's doc comment and
// ApplyOperationalReadiness.
var clusterSpecificCheckIDs = []string{
	"managed-nodegroups",
	"managed-addons",
	"self-managed-addons",
	"workload-health",
	"disruption-readiness",
}

// mismatchOrUnknownReport builds a report carrying cluster-specific findings
// across every affected family (node group rollback warning, add-on
// blocker, workload-health finding, PDB disruption finding, CRD/WH
// findings) plus an EKSCluster identity card the caller can mutate to force
// a match/mismatch/unknown scenario.
func mismatchOrUnknownReport(clusterName, region string) *findings.Report {
	report := cleanOperationalReport()
	if clusterName == "" && region == "" {
		report.EKSCluster = nil
	} else {
		report.EKSCluster = &findings.EKSClusterInfo{ClusterName: clusterName, Region: region}
	}
	report.EKSNodegroups = []findings.EKSNodegroupInfo{{Name: "ng-app", Status: "ACTIVE", Version: "1.35"}}
	report.Findings = []findings.Finding{
		{RuleID: "ADDON-001", Severity: findings.SeverityBlocker, Message: "CoreDNS is incompatible with the rollback target."},
		{RuleID: "WORKLOAD-001", Severity: findings.SeverityWarning, Message: "pod is CrashLoopBackOff"},
		{RuleID: "PDB-001", Severity: findings.SeverityBlocker, Message: "disruptionsAllowed=0"},
		{RuleID: "CRD-001", Severity: findings.SeverityBlocker, Message: "CRD served version removed"},
		{RuleID: "WH-002", Severity: findings.SeverityWarning, Message: "webhook endpoint unavailable"},
	}
	return report
}

// --- 1. Exact name and region match ---

func TestClusterIdentity_ExactMatchPreservesCurrentBehavior(t *testing.T) {
	assessment := eligibleRollbackAssessment() // Cluster "prod"/"ap-south-1"
	report := mismatchOrUnknownReport("prod", "ap-south-1")

	got := ApplyOperationalReadiness(assessment, report)

	if !checkHasReason(got.Checks, "managed-addons", ReasonManagedAddonRollbackRequired) {
		t.Fatalf("managed-addons should consume the ADDON-001 blocker on matching identity: %+v", got.Checks)
	}
	if !checkHasReason(got.Checks, "workload-health", ReasonUnhealthyWorkloadsPresent) {
		t.Fatalf("workload-health should consume WORKLOAD-001 on matching identity: %+v", got.Checks)
	}
	if !checkHasReason(got.Checks, "disruption-readiness", ReasonPDBDisruptionConstraints) {
		t.Fatalf("disruption-readiness should consume PDB-001 on matching identity: %+v", got.Checks)
	}
	if !checkHasReason(got.Checks, "reverse-compatibility", ReasonCRDWebhookControllerRisk) {
		t.Fatalf("reverse-compatibility should consume CRD/WH findings on matching identity: %+v", got.Checks)
	}
	if !checkHasReason(got.Checks, "managed-nodegroups", ReasonManagedNodegroupRollbackRequired) {
		t.Fatalf("managed-nodegroups should consume nodegroup evidence on matching identity: %+v", got.Checks)
	}
	for _, id := range append(clusterSpecificCheckIDs, "reverse-compatibility") {
		if checkHasReason(got.Checks, id, ReasonRollbackEvidenceClusterMismatch) || checkHasReason(got.Checks, id, ReasonRollbackEvidenceClusterUnknown) {
			t.Fatalf("%s unexpectedly carries an identity reason on a matching report: %+v", id, got.Checks)
		}
	}
	if got.Readiness.Status != ReadinessBlocked {
		t.Fatalf("Readiness = %+v, want blocked from genuine matching-identity blockers", got.Readiness)
	}
}

// --- 2/3/4. Mismatch: name, region, and both ---

func TestClusterIdentity_MismatchRoutesAffectedChecksToUnknown(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		region      string
	}{
		{"different name, same region", "staging", "ap-south-1"},
		{"same name, different region", "prod", "us-east-1"},
		{"both different", "staging", "us-east-1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assessment := eligibleRollbackAssessment()
			report := mismatchOrUnknownReport(tc.clusterName, tc.region)

			got := ApplyOperationalReadiness(assessment, report)

			for _, id := range clusterSpecificCheckIDs {
				check := requireRollbackCheck(t, got, id)
				if check.Status != CheckUnknown {
					t.Fatalf("%s status = %s, want unknown for mismatched cluster identity", id, check.Status)
				}
				var reasonCount int
				for _, r := range check.ReasonCodes {
					if r == ReasonRollbackEvidenceClusterMismatch {
						reasonCount++
					}
				}
				if reasonCount != 1 {
					t.Fatalf("%s carries %d %s reasons, want exactly 1 (no duplicates): %+v", id, reasonCount, ReasonRollbackEvidenceClusterMismatch, check.ReasonCodes)
				}
			}
			crdCheck := requireRollbackCheck(t, got, "reverse-compatibility")
			if !checkHasReason(got.Checks, "reverse-compatibility", ReasonRollbackEvidenceClusterMismatch) {
				t.Fatalf("reverse-compatibility missing cluster mismatch reason: %+v", crdCheck)
			}

			// Raw Blocker findings from another cluster must not become a
			// confirmed rollback fail, block readiness, or force
			// do_not_proceed/exit code 2.
			if got.Readiness.Status == ReadinessBlocked {
				t.Fatalf("Readiness = %+v, want not blocked from mismatched cluster-identity evidence alone", got.Readiness)
			}
			if got.Readiness.Blockers != 0 {
				t.Fatalf("Readiness.Blockers = %d, want 0 from mismatched cluster-identity evidence alone", got.Readiness.Blockers)
			}
			recommended := ApplyRecommendation(got)
			if recommended.Recommendation.Decision == RecommendationDoNotProceed {
				t.Fatalf("Recommendation = %q, want mismatch alone to not force do_not_proceed", recommended.Recommendation.Decision)
			}
			if exitCodeFor(recommended) == 2 {
				t.Fatalf("exit code = 2, want non-2 for mismatched cluster-identity evidence alone")
			}
		})
	}
}

// --- 5/6. Missing findings cluster name / region (live report) ---

func TestClusterIdentity_MissingFindingsClusterNameIsUnknown(t *testing.T) {
	assessment := eligibleRollbackAssessment()
	report := mismatchOrUnknownReport("", "ap-south-1")

	got := ApplyOperationalReadiness(assessment, report)
	for _, id := range clusterSpecificCheckIDs {
		check := requireRollbackCheck(t, got, id)
		if check.Status != CheckUnknown {
			t.Fatalf("%s status = %s, want unknown for missing findings cluster name", id, check.Status)
		}
	}
	if !checkHasReason(got.Checks, "managed-addons", ReasonRollbackEvidenceClusterUnknown) {
		t.Fatalf("managed-addons missing %s: %+v", ReasonRollbackEvidenceClusterUnknown, got.Checks)
	}
	if got.Readiness.Status == ReadinessBlocked {
		t.Fatalf("Readiness = %+v, want not blocked from unknown cluster identity alone", got.Readiness)
	}
}

func TestClusterIdentity_MissingFindingsRegionIsUnknown(t *testing.T) {
	assessment := eligibleRollbackAssessment()
	report := mismatchOrUnknownReport("prod", "")

	got := ApplyOperationalReadiness(assessment, report)
	for _, id := range clusterSpecificCheckIDs {
		check := requireRollbackCheck(t, got, id)
		if check.Status != CheckUnknown {
			t.Fatalf("%s status = %s, want unknown for missing findings region", id, check.Status)
		}
	}
	if !checkHasReason(got.Checks, "disruption-readiness", ReasonRollbackEvidenceClusterUnknown) {
		t.Fatalf("disruption-readiness missing %s: %+v", ReasonRollbackEvidenceClusterUnknown, got.Checks)
	}
}

// --- 7. Missing assessed cluster identity ---

func TestClusterIdentity_MissingAssessedClusterIdentityIsConservativeUnknown(t *testing.T) {
	assessment := eligibleRollbackAssessment()
	assessment.Cluster.Region = ""
	report := mismatchOrUnknownReport("prod", "ap-south-1")

	got := ApplyOperationalReadiness(assessment, report)
	for _, id := range clusterSpecificCheckIDs {
		check := requireRollbackCheck(t, got, id)
		if check.Status != CheckUnknown {
			t.Fatalf("%s status = %s, want unknown when assessed cluster region is missing", id, check.Status)
		}
	}
	if !checkHasReason(got.Checks, "workload-health", ReasonRollbackEvidenceClusterUnknown) {
		t.Fatalf("workload-health missing %s: %+v", ReasonRollbackEvidenceClusterUnknown, got.Checks)
	}
}

// --- 8. Whitespace normalization ---

func TestClusterIdentity_WhitespaceIsTrimmedBeforeComparison(t *testing.T) {
	assessment := eligibleRollbackAssessment() // "prod" / "ap-south-1"
	report := mismatchOrUnknownReport("  prod  ", " ap-south-1 ")

	got := ApplyOperationalReadiness(assessment, report)
	if !checkHasReason(got.Checks, "managed-addons", ReasonManagedAddonRollbackRequired) {
		t.Fatalf("whitespace-padded but otherwise matching identity should be consumed normally: %+v", got.Checks)
	}
	if checkHasReason(got.Checks, "managed-addons", ReasonRollbackEvidenceClusterMismatch) ||
		checkHasReason(got.Checks, "managed-addons", ReasonRollbackEvidenceClusterUnknown) {
		t.Fatalf("whitespace-padded matching identity incorrectly flagged: %+v", got.Checks)
	}
}

// --- 9. Manifest-only findings ---

func TestClusterIdentity_ManifestOnlyReportIsNotApplicableNotMismatch(t *testing.T) {
	assessment := eligibleRollbackAssessment()
	report := manifestOnlyOperationalReport()
	// DRAIN-001 legitimately produces manifest-plane findings without any
	// live cluster (see internal/rules/drain001.go's drain001ManifestFinding)
	// -- identity-independent evidence like this must remain available.
	report.Findings = []findings.Finding{{
		RuleID:   "DRAIN-001",
		Severity: findings.SeverityWarning,
		Message:  "manifest-plane singleton replica finding",
	}}

	got := ApplyOperationalReadiness(assessment, report)

	for _, id := range append(clusterSpecificCheckIDs, "reverse-compatibility") {
		if checkHasReason(got.Checks, id, ReasonRollbackEvidenceClusterMismatch) {
			t.Fatalf("%s incorrectly flagged manifest-only report as a wrong-cluster report: %+v", id, got.Checks)
		}
	}
	// A manifest-only report has no live nodegroup/add-on/workload
	// inventory or findings to wrongly confirm -- these checks stay Pass,
	// not Unknown, since there's nothing live-cluster-specific to gate.
	for _, id := range []string{"managed-nodegroups", "managed-addons", "self-managed-addons", "workload-health"} {
		check := requireRollbackCheck(t, got, id)
		if check.Status != CheckPass {
			t.Fatalf("%s status = %s, want pass (nothing live-cluster-specific in a manifest-only report)", id, check.Status)
		}
	}
	disruption := requireRollbackCheck(t, got, "disruption-readiness")
	if !checkEvidenceContains(disruption, "manifest-plane singleton replica finding") {
		t.Fatalf("disruption-readiness should keep identity-independent manifest-plane DRAIN-001 evidence: %+v", disruption)
	}
}

// --- 10. Redacted findings ---

func TestClusterIdentity_RedactedReportStillMatches(t *testing.T) {
	assessment := eligibleRollbackAssessment()
	report := cleanOperationalReport()
	report.EKSCluster.ARN = "arn:aws:eks:ap-south-1:123456789012:cluster/prod"
	report.Findings = []findings.Finding{{
		RuleID:   "ADDON-001",
		Severity: findings.SeverityBlocker,
		Message:  "CoreDNS is incompatible with the rollback target.",
	}}

	redactReportForTest(report)
	if report.EKSCluster.ARN != "[redacted-arn]" {
		t.Fatalf("test precondition failed: ARN not redacted, got %q", report.EKSCluster.ARN)
	}
	if report.EKSCluster.ClusterName != "prod" || report.EKSCluster.Region != "ap-south-1" {
		t.Fatalf("test precondition failed: name/region should survive redaction, got %q/%q", report.EKSCluster.ClusterName, report.EKSCluster.Region)
	}

	got := ApplyOperationalReadiness(assessment, report)
	if !checkHasReason(got.Checks, "managed-addons", ReasonManagedAddonRollbackRequired) {
		t.Fatalf("redacted report with matching name/region should still be consumed as confirmed evidence: %+v", got.Checks)
	}
}

// --- 11. Mismatched API target plus mismatched cluster identity ---

func TestClusterIdentity_MismatchedAPITargetPlusMismatchedClusterIdentity(t *testing.T) {
	assessment := eligibleRollbackAssessment() // RollbackTargetVersion "1.34"
	report := cleanOperationalReport()
	report.TargetVersion = "1.36"                                                              // mismatched API target
	report.EKSCluster = &findings.EKSClusterInfo{ClusterName: "staging", Region: "ap-south-1"} // mismatched cluster
	report.Findings = []findings.Finding{{
		RuleID:   "API-001",
		Severity: findings.SeverityBlocker,
		Message:  "forward target 1.36 removed API finding",
	}}

	got := ApplyOperationalReadiness(assessment, report)
	check := requireRollbackCheck(t, got, "reverse-compatibility")
	if check.Status != CheckUnknown {
		t.Fatalf("reverse-compatibility status = %s, want unknown (deterministic, not fail/pass)", check.Status)
	}
	if !checkHasReason(got.Checks, "reverse-compatibility", ReasonRollbackEvidenceTargetMismatch) {
		t.Fatalf("missing API target mismatch reason: %+v", check.ReasonCodes)
	}
	// No cluster-mismatch reason here: there were no CRD-/WH- findings to
	// gate, and applyIdentityGate for reverse-compatibility only fires the
	// cluster reason when identity blocks -- confirm no duplicate/second
	// Unknown check was created for this single ID.
	var unknownCount int
	for _, c := range got.Checks {
		if c.ID == "reverse-compatibility" {
			unknownCount++
		}
	}
	if unknownCount != 1 {
		t.Fatalf("found %d reverse-compatibility checks, want exactly 1 (no duplicate Unknown checks)", unknownCount)
	}
	if got.Readiness.Status == ReadinessBlocked {
		t.Fatalf("Readiness = %+v, want not blocked", got.Readiness)
	}
}

// --- 12. Matching API target but wrong cluster ---

func TestClusterIdentity_MatchingAPITargetButWrongClusterDoesNotConfirmEvidence(t *testing.T) {
	assessment := eligibleRollbackAssessment() // RollbackTargetVersion "1.34"
	report := cleanOperationalReport()         // TargetVersion "1.34" -- matches
	report.EKSCluster = &findings.EKSClusterInfo{ClusterName: "staging", Region: "ap-south-1"}
	report.Findings = []findings.Finding{
		{RuleID: "API-001", Severity: findings.SeverityBlocker, Message: "matching-target API finding from a different cluster's CRD/WH evidence"},
		{RuleID: "CRD-001", Severity: findings.SeverityBlocker, Message: "CRD finding from a different cluster"},
	}

	got := ApplyOperationalReadiness(assessment, report)
	check := requireRollbackCheck(t, got, "reverse-compatibility")
	// API-001 evidence is still trusted (target validation is independent
	// of cluster identity per this PR's documented boundary), but the
	// CRD-001 blocker from the wrong cluster must not be silently confirmed
	// as a fail merely because the API target happened to match -- the
	// combined check must reflect the cluster mismatch.
	if !checkHasReason(got.Checks, "reverse-compatibility", ReasonRollbackEvidenceClusterMismatch) {
		t.Fatalf("reverse-compatibility missing cluster mismatch reason despite matching API target: %+v", check.ReasonCodes)
	}
}

// --- 13. CRD regression ---

func TestClusterIdentity_CRDRegression(t *testing.T) {
	for _, ruleID := range []string{"CRD-001", "CRD-002"} {
		t.Run(ruleID+"/match", func(t *testing.T) {
			assessment := eligibleRollbackAssessment()
			report := cleanOperationalReport()
			report.Findings = []findings.Finding{{RuleID: ruleID, Severity: findings.SeverityBlocker, Message: "matching-identity CRD finding"}}

			got := ApplyOperationalReadiness(assessment, report)
			check := requireRollbackCheck(t, got, "reverse-compatibility")
			if check.Status != CheckFail || got.Readiness.Status != ReadinessBlocked {
				t.Fatalf("%s matching identity -> %s/%+v, want fail/blocked (unchanged from current behavior)", ruleID, check.Status, got.Readiness)
			}
		})
		t.Run(ruleID+"/mismatch", func(t *testing.T) {
			assessment := eligibleRollbackAssessment()
			report := cleanOperationalReport()
			report.EKSCluster = &findings.EKSClusterInfo{ClusterName: "staging", Region: "ap-south-1"}
			report.Findings = []findings.Finding{{RuleID: ruleID, Severity: findings.SeverityBlocker, Message: "wrong-cluster CRD finding"}}

			got := ApplyOperationalReadiness(assessment, report)
			check := requireRollbackCheck(t, got, "reverse-compatibility")
			if check.Status != CheckUnknown {
				t.Fatalf("%s mismatched identity -> %s, want unknown (not fail/warning)", ruleID, check.Status)
			}
			if got.Readiness.Status == ReadinessBlocked {
				t.Fatalf("%s mismatched identity -> Readiness %+v, want not blocked", ruleID, got.Readiness)
			}
		})
	}
}

// --- 14. PDB/DRAIN regression ---

func TestClusterIdentity_PDBDrainRegression(t *testing.T) {
	t.Run("match preserves PR205 routing", func(t *testing.T) {
		assessment := eligibleRollbackAssessment()
		report := cleanOperationalReport()
		report.Findings = []findings.Finding{{
			RuleID:       "PDB-001",
			Severity:     findings.SeverityBlocker,
			UpgradeGate:  findings.UpgradeGateBlock,
			ImpactScopes: []findings.ImpactScope{findings.ImpactScopeNodeDrain},
			Message:      "disruptionsAllowed=0 for forward worker rollout",
		}}

		got := ApplyOperationalReadiness(assessment, report)
		check := requireRollbackCheck(t, got, "disruption-readiness")
		// PR #205 contract: a PDB blocker without confirmed rollback
		// disruption-activation evidence becomes warning, not fail.
		if check.Status != CheckWarning {
			t.Fatalf("matching-identity PDB-001 -> %s, want warning (PR #205 routing unchanged)", check.Status)
		}
		if !checkHasReason(got.Checks, "disruption-readiness", ReasonPDBDisruptionConstraints) {
			t.Fatalf("missing %s: %+v", ReasonPDBDisruptionConstraints, check.ReasonCodes)
		}
	})
	t.Run("mismatch becomes unknown, does not block", func(t *testing.T) {
		assessment := eligibleRollbackAssessment()
		report := cleanOperationalReport()
		report.EKSCluster = &findings.EKSClusterInfo{ClusterName: "staging", Region: "ap-south-1"}
		report.Findings = []findings.Finding{{
			RuleID:   "DRAIN-002",
			Severity: findings.SeverityBlocker,
			Message:  "wrong-cluster drain finding",
		}}

		got := ApplyOperationalReadiness(assessment, report)
		check := requireRollbackCheck(t, got, "disruption-readiness")
		if check.Status != CheckUnknown {
			t.Fatalf("mismatched-identity DRAIN-002 -> %s, want unknown", check.Status)
		}
		if got.Readiness.Status == ReadinessBlocked {
			t.Fatalf("Readiness = %+v, want not blocked from mismatched disruption evidence alone", got.Readiness)
		}
	})
}

// --- 15. Genuine live rollback blocker (trusted provider/eligibility data) ---

func TestClusterIdentity_GenuineProviderBlockerStillBlocksRegardlessOfFindings(t *testing.T) {
	assessment := eligibleRollbackAssessment()
	assessment.Eligibility = Eligibility{Status: EligibilityUnavailable, Source: "amazon-eks", ReasonCodes: []ReasonCode{ReasonRollbackWindowExpired}}
	assessment.Readiness = Readiness{Status: ReadinessReady}
	assessment.Recommendation = Recommendation{Decision: RecommendationOperatorDecisionRequired, Confidence: ConfidenceMedium}

	// Supplied findings carry a mismatched cluster identity -- this PR must
	// not neuter the genuine, independent provider/eligibility blocker.
	report := mismatchOrUnknownReport("staging", "us-east-1")

	got := ApplyOperationalReadiness(assessment, report)
	recommended := ApplyRecommendation(got)
	if recommended.Recommendation.Decision != RecommendationDoNotProceed {
		t.Fatalf("Recommendation = %q, want do_not_proceed from genuine provider eligibility blocker regardless of mismatched findings", recommended.Recommendation.Decision)
	}
	if exitCodeFor(recommended) != 2 {
		t.Fatalf("exit code = %d, want 2 for a genuine provider eligibility blocker", exitCodeFor(recommended))
	}
}

// --- validateClusterEvidenceIdentity unit coverage ---

func TestValidateClusterEvidenceIdentity(t *testing.T) {
	tests := []struct {
		name           string
		report         *findings.Report
		cluster        Cluster
		wantStatus     clusterEvidenceIdentityStatus
		wantReasonCode ReasonCode
	}{
		{
			name:       "match",
			report:     mismatchOrUnknownReport("prod", "ap-south-1"),
			cluster:    Cluster{Name: "prod", Region: "ap-south-1"},
			wantStatus: clusterEvidenceIdentityMatch,
		},
		{
			name:           "mismatch name",
			report:         mismatchOrUnknownReport("staging", "ap-south-1"),
			cluster:        Cluster{Name: "prod", Region: "ap-south-1"},
			wantStatus:     clusterEvidenceIdentityMismatch,
			wantReasonCode: ReasonRollbackEvidenceClusterMismatch,
		},
		{
			name:           "unknown missing findings identity",
			report:         mismatchOrUnknownReport("", "ap-south-1"),
			cluster:        Cluster{Name: "prod", Region: "ap-south-1"},
			wantStatus:     clusterEvidenceIdentityUnknown,
			wantReasonCode: ReasonRollbackEvidenceClusterUnknown,
		},
		{
			name:       "not applicable manifest-only",
			report:     manifestOnlyOperationalReport(),
			cluster:    Cluster{Name: "prod", Region: "ap-south-1"},
			wantStatus: clusterEvidenceIdentityNotApplicable,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := validateClusterEvidenceIdentity(tc.report, tc.cluster)
			if got.status != tc.wantStatus {
				t.Fatalf("status = %v, want %v", got.status, tc.wantStatus)
			}
			if tc.wantReasonCode != "" && got.reason != tc.wantReasonCode {
				t.Fatalf("reason = %v, want %v", got.reason, tc.wantReasonCode)
			}
		})
	}
}

// exitCodeFor mirrors internal/cli's rollbackExitCode mapping without
// importing internal/cli (which would create an import cycle back into
// internal/rollback).
func exitCodeFor(assessment Assessment) int {
	switch assessment.Recommendation.Decision {
	case RecommendationRollbackPreferred:
		return 0
	case RecommendationDoNotProceed:
		return 2
	default:
		return 1
	}
}

// redactReportForTest mirrors redact.Report's ARN redaction without
// importing internal/redact (which imports internal/rollback and would
// create an import cycle). Kept intentionally narrow: only what this test
// needs to prove ARN redaction doesn't remove name/region identity.
func redactReportForTest(r *findings.Report) {
	if r.EKSCluster == nil {
		return
	}
	r.EKSCluster.ARN = "[redacted-arn]"
}
