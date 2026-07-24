package rollback

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

// freshnessEvaluatedAt is the fixed "now" used throughout this file --
// eligibleRollbackAssessment()'s GeneratedAt (2026-07-15T08:00:00Z). Every
// test in this file derives ScannedAt from this fixed instant so freshness
// comparisons are deterministic and never touch time.Now().
var freshnessEvaluatedAt = time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)

// --- 1/2/3/4. Age boundary ---

func TestFindingsFreshness_ExactlyBelow24HoursIsFresh(t *testing.T) {
	assessment := eligibleRollbackAssessment()
	report := mismatchOrUnknownReport("prod", "ap-south-1")
	report.ScannedAt = freshnessEvaluatedAt.Add(-(24*time.Hour - time.Second))

	got := ApplyOperationalReadiness(assessment, report)

	// Existing (pre-freshness-gate) behavior preserved exactly.
	if !checkHasReason(got.Checks, "managed-addons", ReasonManagedAddonRollbackRequired) {
		t.Fatalf("managed-addons should consume the ADDON-001 blocker on fresh evidence: %+v", got.Checks)
	}
	for _, id := range append(clusterSpecificCheckIDs, "reverse-compatibility") {
		if checkHasReason(got.Checks, id, ReasonRollbackEvidenceStale) || checkHasReason(got.Checks, id, ReasonRollbackEvidenceTimestampUnknown) {
			t.Fatalf("%s unexpectedly carries a freshness reason on fresh evidence: %+v", id, got.Checks)
		}
	}
	if got.Readiness.Status != ReadinessBlocked {
		t.Fatalf("Readiness = %+v, want blocked from genuine matching-identity/fresh-evidence blockers", got.Readiness)
	}
}

func TestFindingsFreshness_Exactly24HoursIsFreshInclusiveBoundary(t *testing.T) {
	assessment := eligibleRollbackAssessment()
	report := mismatchOrUnknownReport("prod", "ap-south-1")
	report.ScannedAt = freshnessEvaluatedAt.Add(-24 * time.Hour) // exactly 24h -- inclusive boundary

	got := ApplyOperationalReadiness(assessment, report)

	if !checkHasReason(got.Checks, "managed-addons", ReasonManagedAddonRollbackRequired) {
		t.Fatalf("managed-addons should consume evidence exactly 24h old (inclusive boundary): %+v", got.Checks)
	}
	for _, id := range append(clusterSpecificCheckIDs, "reverse-compatibility") {
		if checkHasReason(got.Checks, id, ReasonRollbackEvidenceStale) {
			t.Fatalf("%s incorrectly flagged exactly-24h-old evidence as stale: %+v", id, got.Checks)
		}
	}

	got2 := validateFindingsFreshness(report, freshnessEvaluatedAt)
	if got2.status != findingsFreshnessFresh {
		t.Fatalf("validateFindingsFreshness at exactly 24h = %v, want fresh", got2.status)
	}
}

func TestFindingsFreshness_OneSecondOver24HoursIsStale(t *testing.T) {
	assessment := eligibleRollbackAssessment()
	report := mismatchOrUnknownReport("prod", "ap-south-1")
	report.ScannedAt = freshnessEvaluatedAt.Add(-(24*time.Hour + time.Second))

	got := ApplyOperationalReadiness(assessment, report)

	for _, id := range clusterSpecificCheckIDs {
		check := requireRollbackCheck(t, got, id)
		if check.Status != CheckUnknown {
			t.Fatalf("%s status = %s, want unknown for stale (24h+1s) evidence", id, check.Status)
		}
	}
	if !checkHasReason(got.Checks, "managed-addons", ReasonRollbackEvidenceStale) {
		t.Fatalf("managed-addons missing %s: %+v", ReasonRollbackEvidenceStale, got.Checks)
	}
}

func TestFindingsFreshness_VeryOldReportIsStale(t *testing.T) {
	report := mismatchOrUnknownReport("prod", "ap-south-1")
	report.ScannedAt = freshnessEvaluatedAt.Add(-21 * 24 * time.Hour) // three weeks old

	got := validateFindingsFreshness(report, freshnessEvaluatedAt)
	if got.status != findingsFreshnessStale {
		t.Fatalf("status = %v, want stale for a three-week-old report", got.status)
	}
	if got.reason != ReasonRollbackEvidenceStale {
		t.Fatalf("reason = %v, want %v", got.reason, ReasonRollbackEvidenceStale)
	}
}

// --- 5/6/7. Missing or untrusted timestamps ---

func TestFindingsFreshness_ZeroScannedAtIsTimestampUnknown(t *testing.T) {
	report := mismatchOrUnknownReport("prod", "ap-south-1")
	report.ScannedAt = time.Time{}

	got := validateFindingsFreshness(report, freshnessEvaluatedAt)
	if got.status != findingsFreshnessUnknown {
		t.Fatalf("status = %v, want unknown for zero ScannedAt", got.status)
	}
	if got.reason != ReasonRollbackEvidenceTimestampUnknown {
		t.Fatalf("reason = %v, want %v", got.reason, ReasonRollbackEvidenceTimestampUnknown)
	}

	assessment := eligibleRollbackAssessment()
	appliedGot := ApplyOperationalReadiness(assessment, report)
	for _, id := range clusterSpecificCheckIDs {
		check := requireRollbackCheck(t, appliedGot, id)
		if check.Status != CheckUnknown {
			t.Fatalf("%s status = %s, want unknown for missing ScannedAt", id, check.Status)
		}
	}
}

func TestFindingsFreshness_MissingAssessmentGeneratedAtIsConservativeUnknown(t *testing.T) {
	report := mismatchOrUnknownReport("prod", "ap-south-1")
	report.ScannedAt = freshnessEvaluatedAt // otherwise perfectly fresh

	got := validateFindingsFreshness(report, time.Time{})
	if got.status != findingsFreshnessUnknown {
		t.Fatalf("status = %v, want unknown when evaluatedAt (assessment.GeneratedAt) is zero", got.status)
	}
	if got.reason != ReasonRollbackEvidenceTimestampUnknown {
		t.Fatalf("reason = %v, want %v", got.reason, ReasonRollbackEvidenceTimestampUnknown)
	}
}

// TestFindingsFreshness_UntrustedTimestampFromPartialJSON covers a
// findings.json decoded from disk that never populated scannedAt (e.g. a
// hand-edited or pre-freshness-aware file) -- the zero value that results
// from json.Unmarshal must be treated identically to an explicitly zeroed
// ScannedAt, never silently trusted as "now" or as fresh.
func TestFindingsFreshness_UntrustedTimestampFromPartialJSON(t *testing.T) {
	raw := []byte(`{"schemaVersion":"1.0","targetVersion":"1.34","clusterContext":"prod","provider":"eks","findings":[]}`)
	var report findings.Report
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !report.ScannedAt.IsZero() {
		t.Fatalf("test precondition failed: expected zero ScannedAt from JSON missing the field, got %v", report.ScannedAt)
	}

	got := validateFindingsFreshness(&report, freshnessEvaluatedAt)
	if got.status != findingsFreshnessUnknown {
		t.Fatalf("status = %v, want unknown for an untrusted/missing scannedAt from partial JSON", got.status)
	}
	if got.reason != ReasonRollbackEvidenceTimestampUnknown {
		t.Fatalf("reason = %v, want %v", got.reason, ReasonRollbackEvidenceTimestampUnknown)
	}
}

// --- 8/9. Future timestamps and clock skew ---

func TestFindingsFreshness_FutureTimestampWithinToleranceIsFreshWithoutNegativeAge(t *testing.T) {
	report := mismatchOrUnknownReport("prod", "ap-south-1")
	report.ScannedAt = freshnessEvaluatedAt.Add(4 * time.Minute) // within the 5-minute tolerance

	got := validateFindingsFreshness(report, freshnessEvaluatedAt)
	if got.status != findingsFreshnessFresh {
		t.Fatalf("status = %v, want fresh for a future timestamp within the 5-minute skew tolerance", got.status)
	}
	for _, e := range got.evidence {
		if strings.Contains(e, "-") {
			t.Fatalf("fresh evidence unexpectedly contains a negative sign: %q", e)
		}
	}
}

func TestFindingsFreshness_FutureTimestampBeyondToleranceIsTimestampUnknown(t *testing.T) {
	report := mismatchOrUnknownReport("prod", "ap-south-1")
	report.ScannedAt = freshnessEvaluatedAt.Add(10 * time.Minute) // beyond the 5-minute tolerance

	got := validateFindingsFreshness(report, freshnessEvaluatedAt)
	if got.status != findingsFreshnessUnknown {
		t.Fatalf("status = %v, want unknown for a future timestamp beyond the 5-minute skew tolerance", got.status)
	}
	if got.reason != ReasonRollbackEvidenceTimestampUnknown {
		t.Fatalf("reason = %v, want %v", got.reason, ReasonRollbackEvidenceTimestampUnknown)
	}
	found := false
	for _, e := range got.evidence {
		if strings.Contains(e, "clock skew") {
			found = true
		}
		// This evidence path never prints an age -- only the two
		// timestamps and the tolerance -- so there is no negative-age
		// value to guard against here.
	}
	if !found {
		t.Fatalf("evidence should clearly state future-clock-skew detection: %+v", got.evidence)
	}
}

// --- 10. Stale workload Blocker does not become a rollback fail ---

func TestFindingsFreshness_StaleWorkloadBlockerDoesNotBecomeFailOrBlock(t *testing.T) {
	assessment := eligibleRollbackAssessment()
	report := cleanOperationalReport()
	report.ScannedAt = freshnessEvaluatedAt.Add(-48 * time.Hour) // stale
	report.Findings = []findings.Finding{{
		RuleID:   "WORKLOAD-001",
		Severity: findings.SeverityBlocker,
		Message:  "pod is CrashLoopBackOff",
	}}

	got := ApplyOperationalReadiness(assessment, report)
	check := requireRollbackCheck(t, got, "workload-health")
	if check.Status != CheckUnknown {
		t.Fatalf("workload-health status = %s, want unknown for stale Blocker evidence, not fail", check.Status)
	}
	if got.Readiness.Status == ReadinessBlocked {
		t.Fatalf("Readiness = %+v, want not blocked from stale evidence alone", got.Readiness)
	}
	recommended := ApplyRecommendation(got)
	if recommended.Recommendation.Decision == RecommendationDoNotProceed {
		t.Fatalf("Recommendation = %q, want stale evidence alone to not force do_not_proceed", recommended.Recommendation.Decision)
	}
	if exitCodeFor(recommended) == 2 {
		t.Fatalf("exit code = 2, want non-2 for stale evidence alone")
	}
}

// --- 11. Stale PDB/DRAIN evidence ---

func TestFindingsFreshness_StaleDisruptionEvidenceIsUnknown(t *testing.T) {
	assessment := eligibleRollbackAssessment()
	report := cleanOperationalReport()
	report.ScannedAt = freshnessEvaluatedAt.Add(-48 * time.Hour)
	report.Findings = []findings.Finding{{
		RuleID:   "PDB-001",
		Severity: findings.SeverityBlocker,
		Message:  "disruptionsAllowed=0",
	}}

	got := ApplyOperationalReadiness(assessment, report)
	check := requireRollbackCheck(t, got, "disruption-readiness")
	if check.Status != CheckUnknown {
		t.Fatalf("disruption-readiness status = %s, want unknown for stale PDB evidence", check.Status)
	}
	if !checkHasReason(got.Checks, "disruption-readiness", ReasonRollbackEvidenceStale) {
		t.Fatalf("disruption-readiness missing %s: %+v", ReasonRollbackEvidenceStale, check.ReasonCodes)
	}
}

// --- 12. Stale CRD/webhook evidence ---

func TestFindingsFreshness_StaleCRDWebhookEvidenceIsUnknown(t *testing.T) {
	assessment := eligibleRollbackAssessment()
	report := cleanOperationalReport()
	report.ScannedAt = freshnessEvaluatedAt.Add(-48 * time.Hour)
	report.Findings = []findings.Finding{{
		RuleID:   "CRD-001",
		Severity: findings.SeverityBlocker,
		Message:  "CRD served version removed",
	}}

	got := ApplyOperationalReadiness(assessment, report)
	check := requireRollbackCheck(t, got, "reverse-compatibility")
	if check.Status != CheckUnknown {
		t.Fatalf("reverse-compatibility status = %s, want unknown (not fail/warning) for stale CRD evidence", check.Status)
	}
	if !checkHasReason(got.Checks, "reverse-compatibility", ReasonRollbackEvidenceStale) {
		t.Fatalf("reverse-compatibility missing %s: %+v", ReasonRollbackEvidenceStale, check.ReasonCodes)
	}
}

// --- 13. Stale managed/self-managed add-on evidence ---

func TestFindingsFreshness_StaleAddonEvidenceIsUnknown(t *testing.T) {
	assessment := eligibleRollbackAssessment()
	report := cleanOperationalReport()
	report.ScannedAt = freshnessEvaluatedAt.Add(-48 * time.Hour)
	report.Findings = []findings.Finding{
		{RuleID: "ADDON-001", Severity: findings.SeverityBlocker, Message: "CoreDNS is incompatible with the rollback target."},
		{RuleID: "ADDON-002", Severity: findings.SeverityWarning, Message: "self-managed add-on compatibility could not be verified"},
	}

	got := ApplyOperationalReadiness(assessment, report)
	for _, id := range []string{"managed-addons", "self-managed-addons"} {
		check := requireRollbackCheck(t, got, id)
		if check.Status != CheckUnknown {
			t.Fatalf("%s status = %s, want unknown for stale add-on evidence", id, check.Status)
		}
		if !checkHasReason(got.Checks, id, ReasonRollbackEvidenceStale) {
			t.Fatalf("%s missing %s: %+v", id, ReasonRollbackEvidenceStale, check.ReasonCodes)
		}
	}
}

// --- 14. Stale node-group evidence ---

func TestFindingsFreshness_StaleNodegroupEvidenceIsUnknown(t *testing.T) {
	assessment := eligibleRollbackAssessment()
	report := cleanOperationalReport()
	report.ScannedAt = freshnessEvaluatedAt.Add(-48 * time.Hour)
	report.EKSNodegroups = []findings.EKSNodegroupInfo{{Name: "ng-app", Status: "ACTIVE", Version: "1.35"}}

	got := ApplyOperationalReadiness(assessment, report)
	check := requireRollbackCheck(t, got, "managed-nodegroups")
	if check.Status != CheckUnknown {
		t.Fatalf("managed-nodegroups status = %s, want unknown for stale nodegroup evidence", check.Status)
	}
	if !checkHasReason(got.Checks, "managed-nodegroups", ReasonRollbackEvidenceStale) {
		t.Fatalf("managed-nodegroups missing %s: %+v", ReasonRollbackEvidenceStale, check.ReasonCodes)
	}
}

// --- 15. Manifest-only report ---

func TestFindingsFreshness_ManifestOnlyReportIsNotApplicable(t *testing.T) {
	assessment := eligibleRollbackAssessment()
	report := manifestOnlyOperationalReport()
	report.ScannedAt = freshnessEvaluatedAt.Add(-21 * 24 * time.Hour) // would be very stale if it mattered
	report.Findings = []findings.Finding{{
		RuleID:   "DRAIN-001",
		Severity: findings.SeverityWarning,
		Message:  "manifest-plane singleton replica finding",
	}}

	got := validateFindingsFreshness(report, freshnessEvaluatedAt)
	if got.status != findingsFreshnessNotApplicable {
		t.Fatalf("status = %v, want not applicable for a manifest-only report regardless of ScannedAt age", got.status)
	}
	if got.reason != "" || len(got.evidence) != 0 {
		t.Fatalf("not-applicable freshness should carry no reason/evidence: %+v", got)
	}

	applied := ApplyOperationalReadiness(assessment, report)
	for _, id := range append(clusterSpecificCheckIDs, "reverse-compatibility") {
		if checkHasReason(applied.Checks, id, ReasonRollbackEvidenceStale) || checkHasReason(applied.Checks, id, ReasonRollbackEvidenceTimestampUnknown) {
			t.Fatalf("%s incorrectly carries a stale/unknown-timestamp reason for a manifest-only report: %+v", id, applied.Checks)
		}
	}
	disruption := requireRollbackCheck(t, applied, "disruption-readiness")
	if !checkEvidenceContains(disruption, "manifest-plane singleton replica finding") {
		t.Fatalf("disruption-readiness should keep identity-independent manifest-plane DRAIN-001 evidence: %+v", disruption)
	}
}

// --- 16. Redacted report ---

func TestFindingsFreshness_RedactedReportTimestampBehaviorUnchanged(t *testing.T) {
	assessment := eligibleRollbackAssessment()
	report := cleanOperationalReport()
	report.EKSCluster.ARN = "arn:aws:eks:ap-south-1:123456789012:cluster/prod"
	report.ScannedAt = freshnessEvaluatedAt.Add(-48 * time.Hour) // stale
	report.Findings = []findings.Finding{{
		RuleID:   "ADDON-001",
		Severity: findings.SeverityBlocker,
		Message:  "CoreDNS is incompatible with the rollback target.",
	}}

	redactReportForTest(report)
	if report.EKSCluster.ARN != "[redacted-arn]" {
		t.Fatalf("test precondition failed: ARN not redacted, got %q", report.EKSCluster.ARN)
	}

	got := ApplyOperationalReadiness(assessment, report)
	check := requireRollbackCheck(t, got, "managed-addons")
	if check.Status != CheckUnknown {
		t.Fatalf("managed-addons status = %s, want unknown -- redaction must not change stale-evidence routing", check.Status)
	}
	if !checkHasReason(got.Checks, "managed-addons", ReasonRollbackEvidenceStale) {
		t.Fatalf("managed-addons missing %s after redaction: %+v", ReasonRollbackEvidenceStale, check.ReasonCodes)
	}
}

// --- 17. Fresh but cluster-mismatched ---

func TestFindingsFreshness_FreshButClusterMismatchedKeepsIdentityReasonOnly(t *testing.T) {
	assessment := eligibleRollbackAssessment()
	report := mismatchOrUnknownReport("staging", "us-east-1") // mismatched
	report.ScannedAt = freshnessEvaluatedAt                   // perfectly fresh

	got := ApplyOperationalReadiness(assessment, report)
	for _, id := range clusterSpecificCheckIDs {
		check := requireRollbackCheck(t, got, id)
		if check.Status != CheckUnknown {
			t.Fatalf("%s status = %s, want unknown for mismatched identity", id, check.Status)
		}
		if !checkHasReason(got.Checks, id, ReasonRollbackEvidenceClusterMismatch) {
			t.Fatalf("%s missing %s: %+v", id, ReasonRollbackEvidenceClusterMismatch, check.ReasonCodes)
		}
		if checkHasReason(got.Checks, id, ReasonRollbackEvidenceStale) || checkHasReason(got.Checks, id, ReasonRollbackEvidenceTimestampUnknown) {
			t.Fatalf("%s incorrectly carries a freshness reason despite fresh evidence: %+v", id, check.ReasonCodes)
		}
	}
}

// --- 18. Matching cluster but stale ---

func TestFindingsFreshness_MatchingClusterButStaleKeepsStaleReasonOnly(t *testing.T) {
	assessment := eligibleRollbackAssessment()
	report := mismatchOrUnknownReport("prod", "ap-south-1") // matches
	report.ScannedAt = freshnessEvaluatedAt.Add(-48 * time.Hour)

	got := ApplyOperationalReadiness(assessment, report)
	for _, id := range clusterSpecificCheckIDs {
		check := requireRollbackCheck(t, got, id)
		if check.Status != CheckUnknown {
			t.Fatalf("%s status = %s, want unknown for stale evidence", id, check.Status)
		}
		if !checkHasReason(got.Checks, id, ReasonRollbackEvidenceStale) {
			t.Fatalf("%s missing %s: %+v", id, ReasonRollbackEvidenceStale, check.ReasonCodes)
		}
		if checkHasReason(got.Checks, id, ReasonRollbackEvidenceClusterMismatch) || checkHasReason(got.Checks, id, ReasonRollbackEvidenceClusterUnknown) {
			t.Fatalf("%s incorrectly carries an identity reason despite matching cluster: %+v", id, check.ReasonCodes)
		}
	}
}

// --- 19. Stale plus cluster mismatch: one check, both reasons, no duplicates ---

func TestFindingsFreshness_StalePlusClusterMismatchRetainsBothReasonsNoDuplicates(t *testing.T) {
	assessment := eligibleRollbackAssessment()
	report := mismatchOrUnknownReport("staging", "us-east-1") // mismatched
	report.ScannedAt = freshnessEvaluatedAt.Add(-48 * time.Hour)

	got := ApplyOperationalReadiness(assessment, report)
	for _, id := range clusterSpecificCheckIDs {
		check := requireRollbackCheck(t, got, id)
		if check.Status != CheckUnknown {
			t.Fatalf("%s status = %s, want unknown", id, check.Status)
		}
		if !checkHasReason(got.Checks, id, ReasonRollbackEvidenceStale) {
			t.Fatalf("%s missing %s: %+v", id, ReasonRollbackEvidenceStale, check.ReasonCodes)
		}
		if !checkHasReason(got.Checks, id, ReasonRollbackEvidenceClusterMismatch) {
			t.Fatalf("%s missing %s: %+v", id, ReasonRollbackEvidenceClusterMismatch, check.ReasonCodes)
		}
		var checkCount, staleCount, mismatchCount int
		for _, c := range got.Checks {
			if c.ID != id {
				continue
			}
			checkCount++
			for _, r := range c.ReasonCodes {
				if r == ReasonRollbackEvidenceStale {
					staleCount++
				}
				if r == ReasonRollbackEvidenceClusterMismatch {
					mismatchCount++
				}
			}
		}
		if checkCount != 1 {
			t.Fatalf("%s appears %d times, want exactly 1 canonical check", id, checkCount)
		}
		if staleCount != 1 || mismatchCount != 1 {
			t.Fatalf("%s reason counts stale=%d mismatch=%d, want exactly 1 each (no duplicates)", id, staleCount, mismatchCount)
		}
	}
	if got.Readiness.Status == ReadinessBlocked {
		t.Fatalf("Readiness = %+v, want not blocked from stale+mismatched evidence alone", got.Readiness)
	}
}

// --- 20. Stale plus API target mismatch on reverse-compatibility ---

func TestFindingsFreshness_StalePlusAPITargetMismatchIsOneCanonicalUnknownCheck(t *testing.T) {
	assessment := eligibleRollbackAssessment() // RollbackTargetVersion "1.34"
	report := cleanOperationalReport()
	report.TargetVersion = "1.36" // mismatched API target
	report.ScannedAt = freshnessEvaluatedAt.Add(-48 * time.Hour)
	report.Findings = []findings.Finding{{
		RuleID:   "API-001",
		Severity: findings.SeverityBlocker,
		Message:  "forward target 1.36 removed API finding",
	}}

	got := ApplyOperationalReadiness(assessment, report)
	check := requireRollbackCheck(t, got, "reverse-compatibility")
	if check.Status != CheckUnknown {
		t.Fatalf("reverse-compatibility status = %s, want unknown", check.Status)
	}
	if !checkHasReason(got.Checks, "reverse-compatibility", ReasonRollbackEvidenceTargetMismatch) {
		t.Fatalf("missing API target mismatch reason: %+v", check.ReasonCodes)
	}
	if !checkHasReason(got.Checks, "reverse-compatibility", ReasonRollbackEvidenceStale) {
		t.Fatalf("missing stale reason: %+v", check.ReasonCodes)
	}
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

// --- 21. Genuine live provider blocker still blocks despite stale findings ---

func TestFindingsFreshness_GenuineProviderBlockerStillBlocksRegardlessOfStaleFindings(t *testing.T) {
	assessment := eligibleRollbackAssessment()
	assessment.Eligibility = Eligibility{Status: EligibilityUnavailable, Source: "amazon-eks", ReasonCodes: []ReasonCode{ReasonRollbackWindowExpired}}
	assessment.Readiness = Readiness{Status: ReadinessReady}
	assessment.Recommendation = Recommendation{Decision: RecommendationOperatorDecisionRequired, Confidence: ConfidenceMedium}

	report := mismatchOrUnknownReport("prod", "ap-south-1")           // matching identity
	report.ScannedAt = freshnessEvaluatedAt.Add(-21 * 24 * time.Hour) // very stale

	got := ApplyOperationalReadiness(assessment, report)
	recommended := ApplyRecommendation(got)
	if recommended.Recommendation.Decision != RecommendationDoNotProceed {
		t.Fatalf("Recommendation = %q, want do_not_proceed from genuine provider eligibility blocker regardless of stale findings", recommended.Recommendation.Decision)
	}
	if exitCodeFor(recommended) != 2 {
		t.Fatalf("exit code = %d, want 2 for a genuine provider eligibility blocker", exitCodeFor(recommended))
	}
}

// --- 23. Schema/fingerprint regression ---

func TestFindingsFreshness_SchemaAndFingerprintRegression(t *testing.T) {
	assessment := eligibleRollbackAssessment()
	report := mismatchOrUnknownReport("prod", "ap-south-1")
	report.ScannedAt = freshnessEvaluatedAt.Add(-48 * time.Hour)

	// Capture finding fingerprints before ApplyOperationalReadiness runs --
	// operational readiness must never mutate findings or their identity.
	before := make([]string, len(report.Findings))
	for i, f := range report.Findings {
		before[i] = f.Fingerprint
	}

	got := ApplyOperationalReadiness(assessment, report)

	after := make([]string, len(report.Findings))
	for i, f := range report.Findings {
		after[i] = f.Fingerprint
	}
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("finding[%d] fingerprint changed from %q to %q", i, before[i], after[i])
		}
	}

	// New reason codes are additive and pass schema validation alongside
	// pre-existing fields (no fields removed/renamed).
	got.Checks = append(got.Checks, Check{
		ID:          "managed-nodegroups",
		Title:       "Managed node groups are compatible with rollback target",
		Status:      CheckUnknown,
		ReasonCodes: []ReasonCode{ReasonRollbackEvidenceStale, ReasonRollbackEvidenceTimestampUnknown},
	})
	if err := got.Validate(); err != nil {
		t.Fatalf("Validate() with new additive reason codes failed: %v", err)
	}

	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded Assessment
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.SchemaVersion != SchemaVersion {
		t.Fatalf("schemaVersion round-trip = %q, want %q", decoded.SchemaVersion, SchemaVersion)
	}
}

// --- validateFindingsFreshness unit coverage ---

func TestValidateFindingsFreshness(t *testing.T) {
	tests := []struct {
		name       string
		report     *findings.Report
		evaluated  time.Time
		wantStatus findingsFreshnessStatus
		wantReason ReasonCode
	}{
		{
			name:       "fresh",
			report:     reportWithScannedAt(freshnessEvaluatedAt.Add(-1 * time.Hour)),
			evaluated:  freshnessEvaluatedAt,
			wantStatus: findingsFreshnessFresh,
		},
		{
			name:       "stale",
			report:     reportWithScannedAt(freshnessEvaluatedAt.Add(-25 * time.Hour)),
			evaluated:  freshnessEvaluatedAt,
			wantStatus: findingsFreshnessStale,
			wantReason: ReasonRollbackEvidenceStale,
		},
		{
			name:       "zero scannedAt",
			report:     reportWithScannedAt(time.Time{}),
			evaluated:  freshnessEvaluatedAt,
			wantStatus: findingsFreshnessUnknown,
			wantReason: ReasonRollbackEvidenceTimestampUnknown,
		},
		{
			name:       "not applicable manifest-only",
			report:     manifestOnlyOperationalReport(),
			evaluated:  freshnessEvaluatedAt,
			wantStatus: findingsFreshnessNotApplicable,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := validateFindingsFreshness(tc.report, tc.evaluated)
			if got.status != tc.wantStatus {
				t.Fatalf("status = %v, want %v", got.status, tc.wantStatus)
			}
			if tc.wantReason != "" && got.reason != tc.wantReason {
				t.Fatalf("reason = %v, want %v", got.reason, tc.wantReason)
			}
		})
	}
}

func reportWithScannedAt(scannedAt time.Time) *findings.Report {
	report := mismatchOrUnknownReport("prod", "ap-south-1")
	report.ScannedAt = scannedAt
	return report
}
