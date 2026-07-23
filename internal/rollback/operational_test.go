package rollback

import (
	"strings"
	"testing"
	"time"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

func TestApplyOperationalReadinessManagedNodegroupWarning(t *testing.T) {
	assessment := eligibleRollbackAssessment()
	report := cleanOperationalReport()
	report.EKSNodegroups = []findings.EKSNodegroupInfo{{
		Name:    "ng-app",
		Status:  "ACTIVE",
		Version: "1.35",
	}}

	got := ApplyOperationalReadiness(assessment, report)
	if got.Readiness.Status != ReadinessHighRisk {
		t.Fatalf("Readiness = %+v, want high risk", got.Readiness)
	}
	if got.Recommendation.Decision != assessment.Recommendation.Decision {
		t.Fatalf("Recommendation decision changed to %q, want untouched %q", got.Recommendation.Decision, assessment.Recommendation.Decision)
	}
	if !checkHasReason(got.Checks, "managed-nodegroups", ReasonManagedNodegroupRollbackRequired) {
		t.Fatalf("managed-nodegroups check missing reason: %+v", got.Checks)
	}
}

func TestApplyOperationalReadinessAddonBlocker(t *testing.T) {
	report := cleanOperationalReport()
	report.Findings = []findings.Finding{{
		RuleID:   "ADDON-001",
		Severity: findings.SeverityBlocker,
		Message:  "CoreDNS is incompatible with the rollback target.",
	}}

	got := ApplyOperationalReadiness(eligibleRollbackAssessment(), report)
	if got.Readiness.Status != ReadinessBlocked || got.Readiness.Blockers == 0 {
		t.Fatalf("Readiness = %+v, want blocked", got.Readiness)
	}
	if !checkHasReason(got.Checks, "managed-addons", ReasonManagedAddonRollbackRequired) {
		t.Fatalf("managed-addons check missing reason: %+v", got.Checks)
	}
}

func TestApplyOperationalReadinessAddonWarningIsHighRiskNotBlocked(t *testing.T) {
	report := cleanOperationalReport()
	report.Findings = []findings.Finding{{
		RuleID:      "ADDON-001",
		Severity:    findings.SeverityWarning,
		UpgradeGate: findings.UpgradeGateOperatorDecision,
		Message:     "CoreDNS compatibility requires operator decision for the selected context.",
	}}

	got := ApplyOperationalReadiness(eligibleRollbackAssessment(), report)
	if got.Readiness.Status != ReadinessHighRisk || got.Readiness.Blockers != 0 {
		t.Fatalf("Readiness = %+v, want high risk without rollback blocker", got.Readiness)
	}
	if !checkHasReason(got.Checks, "managed-addons", ReasonManagedAddonRollbackRequired) {
		t.Fatalf("managed-addons check missing reason: %+v", got.Checks)
	}
}

func TestApplyOperationalReadinessPartialCoverageIsIncomplete(t *testing.T) {
	report := cleanOperationalReport()
	report.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoveragePartial, Errors: []string{"pods: forbidden"}},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageComplete},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageComplete},
	})

	got := ApplyOperationalReadiness(eligibleRollbackAssessment(), report)
	if got.Readiness.Status != ReadinessInsufficientEvidence {
		t.Fatalf("Readiness = %+v, want insufficient evidence", got.Readiness)
	}
	if got.Evidence.Complete {
		t.Fatal("Evidence.Complete = true, want false for partial coverage")
	}
	if !checkHasReason(got.Checks, "evidence-coverage", ReasonObservabilityEvidenceMissing) {
		t.Fatalf("evidence-coverage check missing reason: %+v", got.Checks)
	}
}

func TestApplyOperationalReadinessFindingFamilies(t *testing.T) {
	report := cleanOperationalReport()
	report.Findings = []findings.Finding{
		{RuleID: "WORKLOAD-001", Severity: findings.SeverityWarning, Message: "pod is CrashLoopBackOff"},
		{RuleID: "PDB-001", Severity: findings.SeverityBlocker, Message: "disruptionsAllowed=0"},
		{RuleID: "API-001", Severity: findings.SeverityBlocker, Message: "new-version-only API risk"},
		{RuleID: "WH-002", Severity: findings.SeverityWarning, Message: "webhook endpoint unavailable"},
	}

	got := ApplyOperationalReadiness(eligibleRollbackAssessment(), report)
	if got.Readiness.Status != ReadinessBlocked {
		t.Fatalf("Readiness = %+v, want blocked from API blocker", got.Readiness)
	}
	for _, tc := range []struct {
		id     string
		reason ReasonCode
	}{
		{"workload-health", ReasonUnhealthyWorkloadsPresent},
		{"disruption-readiness", ReasonPDBDisruptionConstraints},
		{"reverse-compatibility", ReasonNewVersionAPIAdoptionRisk},
		{"reverse-compatibility", ReasonCRDWebhookControllerRisk},
	} {
		if !checkHasReason(got.Checks, tc.id, tc.reason) {
			t.Fatalf("%s missing %s: %+v", tc.id, tc.reason, got.Checks)
		}
	}
}

func TestApplyOperationalReadinessNilReportIsIncomplete(t *testing.T) {
	got := ApplyOperationalReadiness(eligibleRollbackAssessment(), nil)
	if got.Readiness.Status != ReadinessInsufficientEvidence {
		t.Fatalf("Readiness = %+v, want insufficient evidence", got.Readiness)
	}
	if !checkHasReason(got.Checks, "operational-evidence", ReasonObservabilityEvidenceMissing) {
		t.Fatalf("operational-evidence check missing reason: %+v", got.Checks)
	}
}

func TestApplyOperationalReadiness_DisruptionBlockerNeedsRollbackActivationEvidence(t *testing.T) {
	report := cleanOperationalReport()
	report.Findings = []findings.Finding{{
		RuleID:      "PDB-001",
		Severity:    findings.SeverityBlocker,
		UpgradeGate: findings.UpgradeGateAllow,
		Message:     "forward finding is allowed by its effective gate",
	}}

	got := ApplyRecommendation(ApplyOperationalReadiness(eligibleRollbackAssessment(), report))
	check := requireRollbackCheck(t, got, "disruption-readiness")
	if check.Status != CheckWarning || got.Readiness.Status != ReadinessHighRisk || got.Readiness.Blockers != 0 {
		t.Fatalf("check/readiness = %s/%+v, want warning/high_risk without rollback disruption activation evidence", check.Status, got.Readiness)
	}
	if got.Recommendation.Decision != RecommendationFixForwardPreferred {
		t.Fatalf("Recommendation = %q, want fix_forward_preferred for disruption warning", got.Recommendation.Decision)
	}
}

func TestApplyOperationalReadiness_DisruptionWarningGateBlockStaysWarning(t *testing.T) {
	report := cleanOperationalReport()
	report.Findings = []findings.Finding{{
		RuleID:      "PDB-001",
		Severity:    findings.SeverityWarning,
		UpgradeGate: findings.UpgradeGateBlock,
		Message:     "forward finding blocks its selected operation",
	}}

	got := ApplyRecommendation(ApplyOperationalReadiness(eligibleRollbackAssessment(), report))
	check := requireRollbackCheck(t, got, "disruption-readiness")
	if check.Status != CheckWarning || got.Readiness.Status != ReadinessHighRisk || got.Readiness.Blockers != 0 {
		t.Fatalf("check/readiness = %s/%+v, want warning/high_risk from raw Warning despite block gate", check.Status, got.Readiness)
	}
	if got.Recommendation.Decision != RecommendationFixForwardPreferred {
		t.Fatalf("Recommendation = %q, want fix_forward_preferred for high-risk warning", got.Recommendation.Decision)
	}
}

func TestApplyOperationalReadiness_DisruptionImpactScopesDoNotProveRollbackActivation(t *testing.T) {
	scopeSets := [][]findings.ImpactScope{
		{findings.ImpactScopeNodeDrain},
		{findings.ImpactScopeWorkloadRestart},
		{findings.ImpactScopeCurrentHealth},
		{findings.ImpactScopeFutureMaintenance},
		nil,
	}
	for _, scopes := range scopeSets {
		report := cleanOperationalReport()
		report.Findings = []findings.Finding{{
			RuleID:       "DRAIN-002",
			Severity:     findings.SeverityBlocker,
			ImpactScopes: scopes,
			Message:      "spare capacity is unavailable",
		}}

		got := ApplyOperationalReadiness(eligibleRollbackAssessment(), report)
		check := requireRollbackCheck(t, got, "disruption-readiness")
		if check.Status != CheckWarning || got.Readiness.Status != ReadinessHighRisk || got.Readiness.Warnings != 1 {
			t.Fatalf("scopes %v -> check/readiness %s/%+v, want warning/high_risk until rollback activation is confirmed", scopes, check.Status, got.Readiness)
		}
	}
}

func TestApplyOperationalReadiness_DisruptionRoutingIgnoresForwardUpgradeContext(t *testing.T) {
	contexts := []findings.UpgradeContext{
		findings.UpgradeContextAuditOnly,
		findings.UpgradeContextControlPlaneOnly,
		findings.UpgradeContextWorkerRollout,
		findings.UpgradeContextFullPlatformUpgrade,
		findings.UpgradeContextWorkloadRestart,
		findings.UpgradeContextUnspecified,
	}
	for _, ctx := range contexts {
		report := cleanOperationalReport()
		report.UpgradeContext = ctx
		report.Findings = []findings.Finding{{
			RuleID:   "DRAIN-001",
			Severity: findings.SeverityWarning,
			Message:  "single replica workload",
		}}

		got := ApplyOperationalReadiness(eligibleRollbackAssessment(), report)
		check := requireRollbackCheck(t, got, "disruption-readiness")
		if check.Status != CheckWarning || got.Readiness.Status != ReadinessHighRisk || got.Readiness.Warnings != 1 {
			t.Fatalf("upgrade context %s -> check/readiness %s/%+v, want unchanged warning/high_risk", ctx, check.Status, got.Readiness)
		}
	}
}

func TestApplyOperationalReadiness_PDBDoesNotFailRollbackWithoutDrainEvidence(t *testing.T) {
	report := cleanOperationalReport()
	report.UpgradeContext = findings.UpgradeContextWorkerRollout
	report.Findings = []findings.Finding{{
		RuleID:      "PDB-001",
		Severity:    findings.SeverityBlocker,
		UpgradeGate: findings.UpgradeGateBlock,
		Message:     "disruptionsAllowed=0 for forward worker rollout",
	}}

	got := ApplyRecommendation(ApplyOperationalReadiness(eligibleRollbackAssessment(), report))
	check := requireRollbackCheck(t, got, "disruption-readiness")
	if check.Status != CheckWarning || got.Readiness.Status != ReadinessHighRisk || got.Readiness.Blockers != 0 {
		t.Fatalf("PDB rollback routing = %s/%+v, want warning/high_risk without rollback drain evidence", check.Status, got.Readiness)
	}
	if got.Recommendation.Decision == RecommendationDoNotProceed {
		t.Fatalf("Recommendation = %q, want non-blocking recommendation without rollback drain evidence", got.Recommendation.Decision)
	}
}

func TestApplyOperationalReadiness_DisruptionSeverityMappingWithoutActivationEvidence(t *testing.T) {
	tests := []struct {
		ruleID     string
		severity   findings.Severity
		wantStatus CheckStatus
		wantReady  ReadinessStatus
	}{
		{"DRAIN-002", findings.SeverityBlocker, CheckWarning, ReadinessHighRisk},
		{"DRAIN-001", findings.SeverityWarning, CheckWarning, ReadinessHighRisk},
		{"DRAIN-003", findings.SeverityWarning, CheckWarning, ReadinessHighRisk},
		{"DRAIN-004", findings.SeverityWarning, CheckWarning, ReadinessHighRisk},
	}
	for _, tc := range tests {
		report := cleanOperationalReport()
		report.Findings = []findings.Finding{{RuleID: tc.ruleID, Severity: tc.severity, Message: tc.ruleID + " current contract"}}

		got := ApplyOperationalReadiness(eligibleRollbackAssessment(), report)
		check := requireRollbackCheck(t, got, "disruption-readiness")
		if check.Status != tc.wantStatus || got.Readiness.Status != tc.wantReady {
			t.Fatalf("%s %s -> %s/%+v, want %s/%s", tc.ruleID, tc.severity, check.Status, got.Readiness, tc.wantStatus, tc.wantReady)
		}
	}
}

func TestApplyOperationalReadiness_Drain005RoutesOnceThroughWorkloadHealth(t *testing.T) {
	report := cleanOperationalReport()
	report.Findings = []findings.Finding{{
		RuleID:   "DRAIN-005",
		Severity: findings.SeverityWarning,
		Message:  "DaemonSet has fewer ready pods than desired",
	}}

	got := ApplyOperationalReadiness(eligibleRollbackAssessment(), report)
	workload := requireRollbackCheck(t, got, "workload-health")
	disruption := requireRollbackCheck(t, got, "disruption-readiness")
	if len(got.Checks) != 9 {
		t.Fatalf("len(Checks) = %d, want current operational check count 9", len(got.Checks))
	}
	if workload.Status != CheckWarning || disruption.Status != CheckPass {
		t.Fatalf("DRAIN-005 check statuses = workload %s disruption %s, want workload warning and disruption pass", workload.Status, disruption.Status)
	}
	if got.Readiness.Status != ReadinessHighRisk || got.Readiness.Warnings != 1 {
		t.Fatalf("Readiness = %+v, want high_risk with one warning check from workload-health routing", got.Readiness)
	}
	if checkEvidenceContains(disruption, "DRAIN-005") {
		t.Fatalf("DRAIN-005 unexpectedly routed through disruption-readiness evidence: %v", disruption.Evidence)
	}
}

func TestApplyOperationalReadiness_CurrentContractAddonSeverityAndImpacts(t *testing.T) {
	tests := []struct {
		name       string
		finding    findings.Finding
		wantStatus CheckStatus
		wantReady  ReadinessStatus
	}{
		{
			name: "warning operator decision",
			finding: findings.Finding{
				RuleID:       "ADDON-001",
				Severity:     findings.SeverityWarning,
				UpgradeGate:  findings.UpgradeGateOperatorDecision,
				ImpactScopes: []findings.ImpactScope{findings.ImpactScopeFutureMaintenance},
				Message:      "add-on compatibility is a forward operator decision",
			},
			wantStatus: CheckWarning,
			wantReady:  ReadinessHighRisk,
		},
		{
			name: "blocker block",
			finding: findings.Finding{
				RuleID:       "ADDON-001",
				Severity:     findings.SeverityBlocker,
				UpgradeGate:  findings.UpgradeGateBlock,
				ImpactScopes: []findings.ImpactScope{findings.ImpactScopeWorkloadRestart},
				Message:      "add-on compatibility blocks the forward context",
			},
			wantStatus: CheckFail,
			wantReady:  ReadinessBlocked,
		},
	}
	for _, tc := range tests {
		report := cleanOperationalReport()
		report.Findings = []findings.Finding{tc.finding}

		got := ApplyOperationalReadiness(eligibleRollbackAssessment(), report)
		check := requireRollbackCheck(t, got, "managed-addons")
		if check.Status != tc.wantStatus || got.Readiness.Status != tc.wantReady {
			t.Fatalf("%s -> %s/%+v, want %s/%s", tc.name, check.Status, got.Readiness, tc.wantStatus, tc.wantReady)
		}
	}
}

// TestApplyOperationalReadiness_APIDirectionalityMatchingTargetPreservesRawSeverity
// is the matching-target contract: when the supplied findings.json
// TargetVersion equals Cluster.RollbackTargetVersion, API-001/API-002
// routing is unchanged from before evidence-target validation existed --
// raw Blocker severity still becomes a confirmed reverse-compatibility fail.
func TestApplyOperationalReadiness_APIDirectionalityMatchingTargetPreservesRawSeverity(t *testing.T) {
	report := cleanOperationalReport() // TargetVersion "1.34", matches RollbackTargetVersion "1.34"
	report.Findings = []findings.Finding{{
		RuleID:   "API-001",
		Severity: findings.SeverityBlocker,
		Message:  "rollback target 1.34 removed API finding",
	}}

	got := ApplyOperationalReadiness(eligibleRollbackAssessment(), report)
	check := requireRollbackCheck(t, got, "reverse-compatibility")
	if check.Status != CheckFail || got.Readiness.Status != ReadinessBlocked {
		t.Fatalf("API-001 matching-target finding -> %s/%+v, want reverse-compatibility fail/blocked", check.Status, got.Readiness)
	}
	if !checkEvidenceContains(check, "rollback target 1.34") {
		t.Fatalf("reverse-compatibility evidence = %v, want matching-target message preserved", check.Evidence)
	}
	if checkHasReason(got.Checks, "reverse-compatibility", ReasonRollbackEvidenceTargetMismatch) ||
		checkHasReason(got.Checks, "reverse-compatibility", ReasonRollbackEvidenceTargetUnknown) {
		t.Fatalf("matching-target evidence unexpectedly flagged as mismatch/unknown: %+v", check.ReasonCodes)
	}

	recommended := ApplyRecommendation(got)
	if recommended.Recommendation.Decision != RecommendationDoNotProceed {
		t.Fatalf("Recommendation = %q, want do_not_proceed for a genuine matching-target API-001 blocker", recommended.Recommendation.Decision)
	}
}

// TestApplyOperationalReadiness_APIDirectionalityMismatchedTargetBecomesUnknown
// is the false-blocker fix: findings.json generated for a different (future
// upgrade) target must not be trusted as confirmed rollback-fail evidence.
func TestApplyOperationalReadiness_APIDirectionalityMismatchedTargetBecomesUnknown(t *testing.T) {
	report := cleanOperationalReport()
	report.TargetVersion = "1.36"
	report.CurrentVersion = "1.35"
	report.Findings = []findings.Finding{{
		RuleID:   "API-001",
		Severity: findings.SeverityBlocker,
		Message:  "forward target 1.36 removed API finding",
	}}

	got := ApplyOperationalReadiness(eligibleRollbackAssessment(), report) // RollbackTargetVersion "1.34"
	check := requireRollbackCheck(t, got, "reverse-compatibility")
	if check.Status != CheckUnknown {
		t.Fatalf("API-001 mismatched-target finding -> check status %s, want unknown", check.Status)
	}
	if got.Readiness.Status != ReadinessInsufficientEvidence || got.Readiness.Blockers != 0 {
		t.Fatalf("Readiness = %+v, want insufficient_evidence with zero blockers from mismatched API evidence alone", got.Readiness)
	}
	if !checkHasReason(got.Checks, "reverse-compatibility", ReasonRollbackEvidenceTargetMismatch) {
		t.Fatalf("reverse-compatibility check missing %s: %+v", ReasonRollbackEvidenceTargetMismatch, check.ReasonCodes)
	}
	if !checkEvidenceContains(check, "1.36") || !checkEvidenceContains(check, "1.34") {
		t.Fatalf("reverse-compatibility evidence = %v, want both supplied (1.36) and rollback target (1.34) versions present", check.Evidence)
	}

	recommended := ApplyRecommendation(got)
	if recommended.Recommendation.Decision == RecommendationDoNotProceed {
		t.Fatalf("Recommendation = %q, want mismatch alone to not force do_not_proceed", recommended.Recommendation.Decision)
	}
}

// TestApplyOperationalReadiness_APIDirectionalityMismatchedTargetAPI002Warning
// mirrors the mismatch case for API-002: a raw Warning must not be treated
// as confirmed rollback evidence either.
func TestApplyOperationalReadiness_APIDirectionalityMismatchedTargetAPI002Warning(t *testing.T) {
	report := cleanOperationalReport()
	report.TargetVersion = "1.36"
	report.CurrentVersion = "1.35"
	report.Findings = []findings.Finding{{
		RuleID:   "API-002",
		Severity: findings.SeverityWarning,
		Message:  "forward target 1.36 deprecated API finding",
	}}

	got := ApplyOperationalReadiness(eligibleRollbackAssessment(), report)
	check := requireRollbackCheck(t, got, "reverse-compatibility")
	if check.Status != CheckUnknown {
		t.Fatalf("API-002 mismatched-target finding -> check status %s, want unknown (not a confirmed warning)", check.Status)
	}
	if !checkHasReason(got.Checks, "reverse-compatibility", ReasonRollbackEvidenceTargetMismatch) {
		t.Fatalf("reverse-compatibility check missing %s: %+v", ReasonRollbackEvidenceTargetMismatch, check.ReasonCodes)
	}
}

// TestApplyOperationalReadiness_APIDirectionalityMissingFindingsTargetIsUnknown
// covers a findings.json with no TargetVersion at all.
func TestApplyOperationalReadiness_APIDirectionalityMissingFindingsTargetIsUnknown(t *testing.T) {
	report := cleanOperationalReport()
	report.TargetVersion = ""
	report.Findings = []findings.Finding{{
		RuleID:   "API-001",
		Severity: findings.SeverityBlocker,
		Message:  "API finding without a recorded findings target version",
	}}

	got := ApplyOperationalReadiness(eligibleRollbackAssessment(), report)
	check := requireRollbackCheck(t, got, "reverse-compatibility")
	if check.Status != CheckUnknown {
		t.Fatalf("missing findings TargetVersion -> check status %s, want unknown", check.Status)
	}
	if !checkHasReason(got.Checks, "reverse-compatibility", ReasonRollbackEvidenceTargetUnknown) {
		t.Fatalf("reverse-compatibility check missing %s: %+v", ReasonRollbackEvidenceTargetUnknown, check.ReasonCodes)
	}
}

// TestApplyOperationalReadiness_APIDirectionalityMissingRollbackTargetIsUnknown
// covers an assessment whose Cluster.RollbackTargetVersion was never
// populated (e.g. provider eligibility evaluation could not determine it).
func TestApplyOperationalReadiness_APIDirectionalityMissingRollbackTargetIsUnknown(t *testing.T) {
	report := cleanOperationalReport()
	report.Findings = []findings.Finding{{
		RuleID:   "API-001",
		Severity: findings.SeverityBlocker,
		Message:  "API finding evaluated with no known rollback target",
	}}

	assessment := eligibleRollbackAssessment()
	assessment.Cluster.RollbackTargetVersion = ""

	got := ApplyOperationalReadiness(assessment, report)
	check := requireRollbackCheck(t, got, "reverse-compatibility")
	if check.Status != CheckUnknown {
		t.Fatalf("missing RollbackTargetVersion -> check status %s, want unknown", check.Status)
	}
	if !checkHasReason(got.Checks, "reverse-compatibility", ReasonRollbackEvidenceTargetUnknown) {
		t.Fatalf("reverse-compatibility check missing %s: %+v", ReasonRollbackEvidenceTargetUnknown, check.ReasonCodes)
	}
	if got.Readiness.Status == ReadinessBlocked {
		t.Fatalf("Readiness = %+v, want unknown rollback target to not block from API evidence alone", got.Readiness)
	}
}

// TestApplyOperationalReadiness_APIDirectionalityEquivalentVersionFormsAreNotMismatches
// covers the repo's existing minor-version normalization: "v1.34" vs "1.34"
// and a patch-qualified form vs a bare minor must both be treated as the
// same target, not a mismatch.
func TestApplyOperationalReadiness_APIDirectionalityEquivalentVersionFormsAreNotMismatches(t *testing.T) {
	tests := []struct {
		name                  string
		findingsTarget        string
		rollbackTargetVersion string
	}{
		{"v-prefixed vs bare", "v1.34", "1.34"},
		{"patch-qualified vs bare minor", "1.34.2", "1.34"},
		{"both patch-qualified", "1.34.2", "1.34.9"},
		{"eks-suffixed vs bare", "v1.34.6-eks-1234567", "1.34"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			report := cleanOperationalReport()
			report.TargetVersion = tc.findingsTarget
			report.Findings = []findings.Finding{{
				RuleID:   "API-001",
				Severity: findings.SeverityBlocker,
				Message:  "equivalent-version-form API finding",
			}}

			assessment := eligibleRollbackAssessment()
			assessment.Cluster.RollbackTargetVersion = tc.rollbackTargetVersion

			got := ApplyOperationalReadiness(assessment, report)
			check := requireRollbackCheck(t, got, "reverse-compatibility")
			if check.Status != CheckFail {
				t.Fatalf("findings target %q vs rollback target %q -> check status %s, want fail (equivalent, not a mismatch)",
					tc.findingsTarget, tc.rollbackTargetVersion, check.Status)
			}
			if checkHasReason(got.Checks, "reverse-compatibility", ReasonRollbackEvidenceTargetMismatch) {
				t.Fatalf("findings target %q vs rollback target %q incorrectly flagged as mismatch", tc.findingsTarget, tc.rollbackTargetVersion)
			}
		})
	}
}

// TestApplyOperationalReadiness_APIDirectionalityDifferentMinorVersionsAreMismatches
// confirms genuinely different Kubernetes minor targets are still detected.
func TestApplyOperationalReadiness_APIDirectionalityDifferentMinorVersionsAreMismatches(t *testing.T) {
	report := cleanOperationalReport()
	report.TargetVersion = "1.35"
	report.Findings = []findings.Finding{{
		RuleID:   "API-002",
		Severity: findings.SeverityWarning,
		Message:  "different-minor-version API finding",
	}}

	got := ApplyOperationalReadiness(eligibleRollbackAssessment(), report) // RollbackTargetVersion "1.34"
	check := requireRollbackCheck(t, got, "reverse-compatibility")
	if check.Status != CheckUnknown {
		t.Fatalf("findings target 1.35 vs rollback target 1.34 -> check status %s, want unknown (mismatch)", check.Status)
	}
	if !checkHasReason(got.Checks, "reverse-compatibility", ReasonRollbackEvidenceTargetMismatch) {
		t.Fatalf("reverse-compatibility check missing %s: %+v", ReasonRollbackEvidenceTargetMismatch, check.ReasonCodes)
	}
}

// TestApplyOperationalReadiness_APIDirectionalityNoAPIFindingsNoMismatchNoise
// confirms that a differing findings/rollback target alone -- with zero
// API-001/API-002 findings to validate -- never manufactures a mismatch
// reason. This mirrors TestApplyOperationalReadiness_CurrentContractCRDDirectionalityUsesProvidedForwardFindings's
// fixture (CRD findings only, mismatched report/rollback targets).
func TestApplyOperationalReadiness_APIDirectionalityNoAPIFindingsNoMismatchNoise(t *testing.T) {
	report := cleanOperationalReport()
	report.TargetVersion = "1.36"
	report.Findings = []findings.Finding{{
		RuleID:   "CRD-001",
		Severity: findings.SeverityBlocker,
		Message:  "forward target CRD finding, no API findings present",
	}}

	got := ApplyOperationalReadiness(eligibleRollbackAssessment(), report) // RollbackTargetVersion "1.34"
	check := requireRollbackCheck(t, got, "reverse-compatibility")
	if checkHasReason(got.Checks, "reverse-compatibility", ReasonRollbackEvidenceTargetMismatch) ||
		checkHasReason(got.Checks, "reverse-compatibility", ReasonRollbackEvidenceTargetUnknown) {
		t.Fatalf("reverse-compatibility check unexpectedly carries an API evidence-target reason with no API findings present: %+v", check.ReasonCodes)
	}
	// The CRD-001 blocker itself is untouched by this PR's scope.
	if check.Status != CheckFail {
		t.Fatalf("CRD-001 forward finding -> check status %s, want fail (CRD routing unchanged)", check.Status)
	}
}

func TestApplyOperationalReadiness_CurrentContractCRDDirectionalityUsesProvidedForwardFindings(t *testing.T) {
	for _, ruleID := range []string{"CRD-001", "CRD-002"} {
		report := cleanOperationalReport()
		report.TargetVersion = "1.36"
		report.CurrentVersion = "1.35"
		report.Findings = []findings.Finding{{
			RuleID:   ruleID,
			Severity: findings.SeverityBlocker,
			Message:  "forward target CRD finding",
		}}

		got := ApplyOperationalReadiness(eligibleRollbackAssessment(), report)
		check := requireRollbackCheck(t, got, "reverse-compatibility")
		if check.Status != CheckFail || got.Readiness.Status != ReadinessBlocked {
			t.Fatalf("%s forward finding -> %s/%+v, want reverse-compatibility fail/blocked without rollback-target recalculation", ruleID, check.Status, got.Readiness)
		}
	}
}

func TestApplyOperationalReadiness_CurrentContractUnmappedRulesAreIgnored(t *testing.T) {
	for _, ruleID := range []string{"NODE-001", "NODE-002", "NODE-003", "NET-002", "APISERVICE-001", "COREDNS-001"} {
		report := cleanOperationalReport()
		report.Findings = []findings.Finding{{
			RuleID:   ruleID,
			Severity: findings.SeverityBlocker,
			Message:  ruleID + " is not currently routed by rollback operational readiness",
		}}

		got := ApplyOperationalReadiness(eligibleRollbackAssessment(), report)
		if got.Readiness.Status != ReadinessReady || got.Readiness.Blockers != 0 || got.Readiness.Warnings != 0 {
			t.Fatalf("%s -> Readiness %+v, want ignored/ready", ruleID, got.Readiness)
		}
		if checkEvidenceContains(requireRollbackCheck(t, got, "reverse-compatibility"), ruleID) ||
			checkEvidenceContains(requireRollbackCheck(t, got, "disruption-readiness"), ruleID) ||
			checkEvidenceContains(requireRollbackCheck(t, got, "managed-addons"), ruleID) {
			t.Fatalf("%s unexpectedly routed through rollback checks: %+v", ruleID, got.Checks)
		}
	}
}

func eligibleRollbackAssessment() Assessment {
	now := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	a := NewAssessment(ModePostUpgradeReadiness, now)
	a.Cluster = Cluster{
		Name:                  "prod",
		Region:                "ap-south-1",
		Provider:              "eks",
		CurrentVersion:        "1.35",
		RollbackTargetVersion: "1.34",
	}
	a.Eligibility = Eligibility{Status: EligibilityEligible, Source: "amazon-eks"}
	a.Readiness = Readiness{Status: ReadinessReady}
	a.Recommendation = Recommendation{Decision: RecommendationOperatorDecisionRequired, Confidence: ConfidenceMedium}
	a.Evidence = Evidence{Complete: true}
	return a
}

func cleanOperationalReport() *findings.Report {
	report := findings.NewReport("1.34", "prod", "eks", time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC), nil)
	report.CurrentVersion = "1.35"
	report.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageComplete},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageComplete},
	})
	report.EKSAddons = []findings.EKSAddonInfo{{Name: "coredns", CurrentVersion: "v1.12.0", Compatible: true}}
	// EKSCluster identity matches eligibleRollbackAssessment()'s
	// Cluster.Name/Region ("prod"/"ap-south-1") -- every pre-existing test
	// built on this fixture assumes cluster-specific evidence is consumed
	// normally, which now requires confirmed matching cluster identity (see
	// validateClusterEvidenceIdentity in operational.go). Tests exercising
	// mismatch/unknown/not-applicable identity build their own report
	// instead of using this shared fixture.
	report.EKSCluster = &findings.EKSClusterInfo{ClusterName: "prod", Region: "ap-south-1"}
	return report
}

// manifestOnlyOperationalReport builds a findings.json shape equivalent to
// `kubepreflight scan --manifests-only`: no kubeconfig was ever loaded
// (ClusterContext empty) and no AWS/EKS enrichment was attempted
// (EKSCluster nil) -- see internal/cli/scan.go's --manifests-only
// validation and eksClusterInfo's nil-when-unavailable contract.
func manifestOnlyOperationalReport() *findings.Report {
	report := findings.NewReport("1.34", "", "", time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC), nil)
	report.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageSkipped},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageSkipped},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageComplete},
	})
	return report
}

func checkHasReason(checks []Check, id string, reason ReasonCode) bool {
	for _, check := range checks {
		if check.ID != id {
			continue
		}
		for _, got := range check.ReasonCodes {
			if got == reason {
				return true
			}
		}
	}
	return false
}

func requireRollbackCheck(t *testing.T, assessment Assessment, id string) Check {
	t.Helper()
	for _, check := range assessment.Checks {
		if check.ID == id {
			return check
		}
	}
	t.Fatalf("check %q not found in %+v", id, assessment.Checks)
	return Check{}
}

func checkEvidenceContains(check Check, want string) bool {
	for _, evidence := range check.Evidence {
		if strings.Contains(evidence, want) {
			return true
		}
	}
	return false
}
