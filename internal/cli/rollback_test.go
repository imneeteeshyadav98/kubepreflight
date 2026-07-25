package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
	"github.com/imneeteeshyadav98/kubepreflight/internal/rollback"
)

func TestRollbackCommandHasPlanAndAssess(t *testing.T) {
	exitCode := 0
	cmd := newRollbackCmd(&exitCode)

	if cmd.Name() != "rollback" {
		t.Fatalf("Name = %q, want rollback", cmd.Name())
	}
	for _, name := range []string{"plan", "assess"} {
		sub, _, err := cmd.Find([]string{name, "--help"})
		if err != nil {
			t.Fatalf("Find(%s): %v", name, err)
		}
		if sub == nil || sub.Name() != name {
			t.Fatalf("Find(%s) = %v", name, sub)
		}
		for _, flag := range []string{"provider", "cluster-name", "output", "assessment-out", "findings", "terminal-output", "collector-timeout"} {
			if sub.Flags().Lookup(flag) == nil {
				t.Fatalf("rollback %s missing --%s flag", name, flag)
			}
		}
	}
}

func TestRollbackReportTargetsAlwaysIncludeAssessmentJSON(t *testing.T) {
	targets := rollbackReportTargets("all", "out", "custom.json")
	got := targetPaths(targets)
	want := []string{
		filepath.Join("out", "custom.json"),
		filepath.Join("out", "rollback-report.md"),
		filepath.Join("out", "rollback-report.html"),
	}
	if len(got) != len(want) {
		t.Fatalf("targets = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("targets = %v, want %v", got, want)
		}
	}
}

func TestRollbackReportTargetsDoesNotDoublePrefixAnAlreadyJoinedAssessmentOut(t *testing.T) {
	// A caller that passes --output-dir and a matching --assessment-out
	// that already includes that same directory (the pattern
	// scripts/live-eks/run-smoke.sh uses, mirroring how scan's
	// --findings-out is invoked) must not get outputDir prepended twice.
	// Found via a real live EKS run: rollback plan failed writing to
	// out/out/rollback-assessment.json, a path that never exists.
	assessmentOut := filepath.Join("out", "rollback-assessment.json")
	targets := rollbackReportTargets("json", "out", assessmentOut)
	got := targetPaths(targets)
	want := []string{assessmentOut}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("targets = %v, want %v", got, want)
	}
}

func TestRollbackExitCodeMapping(t *testing.T) {
	tests := []struct {
		decision rollback.RecommendationDecision
		want     int
	}{
		{rollback.RecommendationRollbackPreferred, 0},
		{rollback.RecommendationFixForwardPreferred, 1},
		{rollback.RecommendationOperatorDecisionRequired, 1},
		{rollback.RecommendationDoNotProceed, 2},
	}
	for _, tc := range tests {
		got := rollbackExitCode(rollback.Assessment{
			Recommendation: rollback.Recommendation{Decision: tc.decision},
		})
		if got != tc.want {
			t.Fatalf("rollbackExitCode(%q) = %d, want %d", tc.decision, got, tc.want)
		}
	}
}

func TestRollbackExitCodeDisruptionOnlyFindingDoesNotReturnTwo(t *testing.T) {
	assessment := rollback.NewAssessment(rollback.ModePostUpgradeReadiness, time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC))
	assessment.Cluster = rollback.Cluster{
		Name:                  "prod",
		Provider:              "eks",
		CurrentVersion:        "1.35",
		RollbackTargetVersion: "1.34",
	}
	assessment.Eligibility = rollback.Eligibility{Status: rollback.EligibilityEligible, Source: "amazon-eks"}
	assessment.Readiness = rollback.Readiness{Status: rollback.ReadinessReady}
	assessment.Recommendation = rollback.Recommendation{Decision: rollback.RecommendationOperatorDecisionRequired, Confidence: rollback.ConfidenceMedium}
	assessment.Evidence = rollback.Evidence{Complete: true}

	report := findings.NewReport("1.34", "prod", "", time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC), []findings.Finding{{
		RuleID:       "PDB-001",
		Severity:     findings.SeverityBlocker,
		UpgradeGate:  findings.UpgradeGateBlock,
		ImpactScopes: []findings.ImpactScope{findings.ImpactScopeNodeDrain},
		Message:      "disruptionsAllowed=0 for forward worker rollout",
	}})
	report.CurrentVersion = "1.35"
	report.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageComplete},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageComplete},
	})

	got := rollback.ApplyRecommendation(rollback.ApplyOperationalReadiness(assessment, report))
	if got.Recommendation.Decision == rollback.RecommendationDoNotProceed {
		t.Fatalf("Recommendation = %q, want non-blocking decision without rollback disruption activation evidence", got.Recommendation.Decision)
	}
	if code := rollbackExitCode(got); code == 2 {
		t.Fatalf("rollbackExitCode = %d, want non-2 for disruption-only finding without activation evidence", code)
	}
}

func TestRollbackExitCodeMatchingAPIEvidenceTargetAllowsDoNotProceed(t *testing.T) {
	assessment := baseRollbackAssessment()

	report := findings.NewReport("1.34", "prod", "", time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC), []findings.Finding{{
		RuleID:   "API-001",
		Severity: findings.SeverityBlocker,
		Message:  "matching rollback target 1.34 removed API finding",
	}})
	report.CurrentVersion = "1.35"
	report.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageComplete},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageComplete},
	})

	got := rollback.ApplyRecommendation(rollback.ApplyOperationalReadiness(assessment, report))
	if got.Recommendation.Decision != rollback.RecommendationDoNotProceed {
		t.Fatalf("Recommendation = %q, want do_not_proceed for a genuine matching-target API-001 blocker", got.Recommendation.Decision)
	}
	if code := rollbackExitCode(got); code != 2 {
		t.Fatalf("rollbackExitCode = %d, want 2 for a genuine matching-target API-001 blocker", code)
	}
}

func TestRollbackExitCodeMismatchedAPIEvidenceTargetDoesNotReturnTwo(t *testing.T) {
	assessment := baseRollbackAssessment() // RollbackTargetVersion "1.34"

	report := findings.NewReport("1.36", "prod", "", time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC), []findings.Finding{{
		RuleID:   "API-001",
		Severity: findings.SeverityBlocker,
		Message:  "forward target 1.36 removed API finding, unrelated to rollback target 1.34",
	}})
	report.CurrentVersion = "1.35"
	report.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageComplete},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageComplete},
	})

	got := rollback.ApplyRecommendation(rollback.ApplyOperationalReadiness(assessment, report))
	if got.Recommendation.Decision == rollback.RecommendationDoNotProceed {
		t.Fatalf("Recommendation = %q, want mismatched API evidence target to not force do_not_proceed", got.Recommendation.Decision)
	}
	if code := rollbackExitCode(got); code == 2 {
		t.Fatalf("rollbackExitCode = %d, want non-2 for mismatched API evidence target alone", code)
	}
}

// TestRollbackExitCodeMismatchedClusterIdentityDoesNotReturnTwo covers this
// PR's core fix: findings.json collected against a different EKS cluster
// (same TargetVersion, so API-001 evidence-target validation alone would
// not have caught this) must not become a confirmed rollback-fail and must
// not force exit code 2.
func TestRollbackExitCodeMismatchedClusterIdentityDoesNotReturnTwo(t *testing.T) {
	assessment := baseRollbackAssessment()
	assessment.Cluster.Region = "ap-south-1"

	report := findings.NewReport("1.34", "prod", "eks", time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC), []findings.Finding{{
		RuleID:   "ADDON-001",
		Severity: findings.SeverityBlocker,
		Message:  "matching rollback target 1.34 removed API finding, but from a different cluster",
	}})
	report.CurrentVersion = "1.35"
	report.EKSCluster = &findings.EKSClusterInfo{ClusterName: "staging", Region: "ap-south-1"} // mismatched cluster name
	report.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageComplete},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageComplete},
	})

	got := rollback.ApplyRecommendation(rollback.ApplyOperationalReadiness(assessment, report))
	if got.Recommendation.Decision == rollback.RecommendationDoNotProceed {
		t.Fatalf("Recommendation = %q, want mismatched cluster identity to not force do_not_proceed", got.Recommendation.Decision)
	}
	if code := rollbackExitCode(got); code == 2 {
		t.Fatalf("rollbackExitCode = %d, want non-2 for mismatched cluster-identity evidence alone", code)
	}
}

// TestRollbackExitCodeGenuineEligibilityBlockerIgnoresFindingsClusterIdentity
// proves this PR does not neuter real rollback blockers: a genuine
// provider/eligibility blocker (independent of --findings entirely) must
// still produce do_not_proceed/exit code 2 even when supplied findings
// happen to carry a mismatched cluster identity.
func TestRollbackExitCodeGenuineEligibilityBlockerIgnoresFindingsClusterIdentity(t *testing.T) {
	assessment := baseRollbackAssessment()
	assessment.Cluster.Region = "ap-south-1"
	assessment.Eligibility = rollback.Eligibility{Status: rollback.EligibilityUnavailable, Source: "amazon-eks", ReasonCodes: []rollback.ReasonCode{rollback.ReasonRollbackWindowExpired}}

	report := findings.NewReport("1.34", "prod", "eks", time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC), nil)
	report.CurrentVersion = "1.35"
	report.EKSCluster = &findings.EKSClusterInfo{ClusterName: "staging", Region: "us-east-1"} // mismatched
	report.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageComplete},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageComplete},
	})

	got := rollback.ApplyRecommendation(rollback.ApplyOperationalReadiness(assessment, report))
	if got.Recommendation.Decision != rollback.RecommendationDoNotProceed {
		t.Fatalf("Recommendation = %q, want do_not_proceed from genuine eligibility blocker regardless of findings cluster identity", got.Recommendation.Decision)
	}
	if code := rollbackExitCode(got); code != 2 {
		t.Fatalf("rollbackExitCode = %d, want 2 for a genuine eligibility blocker", code)
	}
}

// TestRollbackExitCodeStaleFindingsDoesNotReturnTwo covers the findings
// freshness gate: a same-cluster, correct-target findings.json that is
// older than the fixed 24-hour maximum age must not become a confirmed
// rollback-fail and must not force exit code 2, even when it carries a raw
// Blocker finding.
func TestRollbackExitCodeStaleFindingsDoesNotReturnTwo(t *testing.T) {
	assessment := baseRollbackAssessment()
	assessment.Cluster.Region = "ap-south-1"

	report := findings.NewReport("1.34", "prod", "eks", time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC), []findings.Finding{{
		RuleID:   "ADDON-001",
		Severity: findings.SeverityBlocker,
		Message:  "matching rollback target 1.34 removed API finding, but scanned two days ago",
	}})
	report.CurrentVersion = "1.35"
	report.EKSCluster = &findings.EKSClusterInfo{ClusterName: "prod", Region: "ap-south-1"} // matching cluster
	report.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageComplete},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageComplete},
	})

	got := rollback.ApplyRecommendation(rollback.ApplyOperationalReadiness(assessment, report))
	if got.Recommendation.Decision == rollback.RecommendationDoNotProceed {
		t.Fatalf("Recommendation = %q, want stale findings to not force do_not_proceed", got.Recommendation.Decision)
	}
	if code := rollbackExitCode(got); code == 2 {
		t.Fatalf("rollbackExitCode = %d, want non-2 for stale findings evidence alone", code)
	}
}

// TestRollbackExitCodeGenuineEligibilityBlockerIgnoresStaleFindings proves
// this PR does not neuter real rollback blockers: a genuine
// provider/eligibility blocker (independent of --findings entirely) must
// still produce do_not_proceed/exit code 2 even when supplied findings
// happen to be stale.
func TestRollbackExitCodeGenuineEligibilityBlockerIgnoresStaleFindings(t *testing.T) {
	assessment := baseRollbackAssessment()
	assessment.Cluster.Region = "ap-south-1"
	assessment.Eligibility = rollback.Eligibility{Status: rollback.EligibilityUnavailable, Source: "amazon-eks", ReasonCodes: []rollback.ReasonCode{rollback.ReasonRollbackWindowExpired}}

	report := findings.NewReport("1.34", "prod", "eks", time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC), nil) // weeks stale
	report.CurrentVersion = "1.35"
	report.EKSCluster = &findings.EKSClusterInfo{ClusterName: "prod", Region: "ap-south-1"}
	report.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageComplete},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageComplete},
	})

	got := rollback.ApplyRecommendation(rollback.ApplyOperationalReadiness(assessment, report))
	if got.Recommendation.Decision != rollback.RecommendationDoNotProceed {
		t.Fatalf("Recommendation = %q, want do_not_proceed from genuine eligibility blocker regardless of stale findings", got.Recommendation.Decision)
	}
	if code := rollbackExitCode(got); code != 2 {
		t.Fatalf("rollbackExitCode = %d, want 2 for a genuine eligibility blocker", code)
	}
}

// TestRollbackExitCodeHardEligibilityBlockerWithInsufficientEvidenceIsValidAndBlocked
// is the CLI-level proof for the exit-code contract fix: a definitive
// eligibility failure (expired rollback window) combined with
// insufficient_evidence readiness (no --findings, or --findings that failed
// a PR #207/#208/#209 provenance gate) must still produce a structurally
// valid Assessment.Validate() and rollbackExitCode 2 -- the same real-world
// shape internal/rollback/eks's hard_eligibility_test.go exercises through
// the actual collector pipeline. rollbackExitCode is unexported, so this
// assertion lives here rather than exporting it solely for a test.
func TestRollbackExitCodeHardEligibilityBlockerWithInsufficientEvidenceIsValidAndBlocked(t *testing.T) {
	assessment := rollback.NewAssessment(rollback.ModePostUpgradeReadiness, time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC))
	assessment.Cluster = rollback.Cluster{
		Name:                  "prod",
		Region:                "ap-south-1",
		Provider:              "eks",
		CurrentVersion:        "1.35",
		RollbackTargetVersion: "1.34",
	}
	assessment.Eligibility = rollback.Eligibility{
		Status:      rollback.EligibilityUnavailable,
		Source:      "amazon-eks",
		ReasonCodes: []rollback.ReasonCode{rollback.ReasonRollbackWindowExpired},
	}
	assessment.Readiness = rollback.Readiness{Status: rollback.ReadinessInsufficientEvidence, Unknowns: 1}
	assessment.Recommendation = rollback.Recommendation{
		Decision:    rollback.RecommendationDoNotProceed,
		Confidence:  rollback.ConfidenceHigh,
		ReasonCodes: []rollback.ReasonCode{rollback.ReasonRollbackWindowExpired},
	}
	assessment.Evidence = rollback.Evidence{Complete: false}

	if err := assessment.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want a valid assessment for a confirmed hard blocker with incomplete operational evidence", err)
	}
	if code := rollbackExitCode(assessment); code != 2 {
		t.Fatalf("rollbackExitCode = %d, want 2 for a valid do_not_proceed assessment", code)
	}
}

// TestRollbackExitCodeGenericValidationFailureIsNotRemappedToTwo confirms the
// exit-code contract fix does not widen exit code 2 beyond valid
// do_not_proceed assessments: an assessment that fails Assessment.Validate()
// for an unrelated reason (here, an unsupported schema version) never
// reaches rollbackExitCode at all in the real CLI path -- newRollbackCmd
// returns the plain (non-infraFailure) Validate() error first, which
// exitCodeForError maps to the generic exit code 1, exactly like every other
// pre-existing model-validation failure.
func TestRollbackExitCodeGenericValidationFailureIsNotRemappedToTwo(t *testing.T) {
	assessment := baseRollbackAssessment()
	assessment.SchemaVersion = "rollback.kubepreflight.io/v0"

	err := assessment.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want an error for an unsupported schema version")
	}
	if isInfraFailure(err) {
		t.Fatal("Validate() error unexpectedly wrapped as an infra failure")
	}
	if got := exitCodeForError(err, 0); got != 1 {
		t.Fatalf("exitCodeForError(generic Validate() error) = %d, want 1", got)
	}
}

// TestRollbackReportWriteFailuresAreInfraFailures guards the two
// write-path call sites newRollbackAssessmentCmd's shared RunE contains
// (rollback.go): os.MkdirAll(outputDir, ...) and the writeRollbackReportFile
// loop over rollbackReportTargets. Both "plan" and "rollback assess" use
// this exact same RunE (see newRollbackCmd), so this test covers both
// command names explicitly.
//
// Driving these through the real command needs a reachable EKS
// collector (rollbackeks.LoadCollector loads real AWS credentials), which
// is unavailable offline -- matching the same constraint plan_test.go's
// write-helper tests document. This instead builds a synthetic, valid
// rollback.Assessment (via baseRollbackAssessment, the same helper the
// rest of this file's exit-code-mapping tests use) with no AWS/cluster
// access at all, drives the exact production helpers RunE calls
// (rollbackReportTargets, writeRollbackReportFile), and applies the same
// infraFailure(fmt.Errorf("...: %w", err)) wrapping RunE performs at each
// call site to confirm the result classifies as exit 4 -- while confirming
// the assessment itself (recommendation/eligibility) stays valid, proving
// the failure is purely at the write stage, not upstream.
func TestRollbackReportWriteFailuresAreInfraFailures(t *testing.T) {
	for _, cmdName := range []string{"plan", "assess"} {
		t.Run(cmdName, func(t *testing.T) {
			assessment := baseRollbackAssessment()
			if err := assessment.Validate(); err != nil {
				t.Fatalf("synthetic assessment is invalid before any write attempt: %v", err)
			}

			t.Run("output directory creation", func(t *testing.T) {
				dir := t.TempDir()
				outputDir := filepath.Join(dir, "blocked-output")
				if err := os.WriteFile(outputDir, []byte("not a directory"), 0o644); err != nil {
					t.Fatalf("seeding blocking file: %v", err)
				}
				mkdirErr := os.MkdirAll(outputDir, 0o755)
				if mkdirErr == nil {
					t.Fatal("os.MkdirAll succeeded against a file-blocked path, want an error")
				}
				wrapped := infraFailure(fmt.Errorf("creating output directory: %w", mkdirErr))
				if !isInfraFailure(wrapped) {
					t.Errorf("wrapped error = %v, want it marked as an infrastructure failure", wrapped)
				}
				if got := exitCodeForError(wrapped, 0); got != 4 {
					t.Errorf("exitCodeForError = %d, want 4", got)
				}
			})

			t.Run("assessment JSON write failure", func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, "rollback-assessment.json")
				if err := os.Mkdir(path, 0o755); err != nil {
					t.Fatalf("seeding directory at assessment path: %v", err)
				}
				for _, target := range rollbackReportTargets("json", dir, "rollback-assessment.json") {
					writeErr := writeRollbackReportFile(target.path, &assessment, target.write)
					if writeErr == nil {
						t.Fatalf("writeRollbackReportFile(%s) succeeded against a directory target, want an error", target.path)
					}
					wrapped := infraFailure(fmt.Errorf("writing rollback report: %w", writeErr))
					if !isInfraFailure(wrapped) {
						t.Errorf("wrapped error = %v, want it marked as an infrastructure failure", wrapped)
					}
					if got := exitCodeForError(wrapped, 0); got != 4 {
						t.Errorf("exitCodeForError = %d, want 4", got)
					}
				}
			})

			// "all" partial failure: assessment JSON and rollback-report.md
			// succeed, rollback-report.html fails because it already exists
			// as a directory -- earlier artifacts must remain, no atomicity
			// is asserted or expected.
			t.Run("all output partial failure", func(t *testing.T) {
				dir := t.TempDir()
				if err := os.Mkdir(filepath.Join(dir, "rollback-report.html"), 0o755); err != nil {
					t.Fatalf("seeding directory at rollback-report.html path: %v", err)
				}
				var firstErr error
				for _, target := range rollbackReportTargets("all", dir, "rollback-assessment.json") {
					if err := writeRollbackReportFile(target.path, &assessment, target.write); err != nil {
						firstErr = err
						break
					}
				}
				if firstErr == nil {
					t.Fatal("writing all rollback report targets succeeded, want the report.html target to fail")
				}
				wrapped := infraFailure(fmt.Errorf("writing rollback report: %w", firstErr))
				if !isInfraFailure(wrapped) {
					t.Errorf("wrapped error = %v, want it marked as an infrastructure failure", wrapped)
				}
				if got := exitCodeForError(wrapped, 0); got != 4 {
					t.Errorf("exitCodeForError = %d, want 4", got)
				}
				if _, statErr := os.Stat(filepath.Join(dir, "rollback-assessment.json")); statErr != nil {
					t.Errorf("rollback-assessment.json missing after partial failure: %v, want the earlier successful write left in place", statErr)
				}
				if _, statErr := os.Stat(filepath.Join(dir, "rollback-report.md")); statErr != nil {
					t.Errorf("rollback-report.md missing after partial failure: %v, want the earlier successful write left in place", statErr)
				}
				info, statErr := os.Stat(filepath.Join(dir, "rollback-report.html"))
				if statErr != nil {
					t.Fatalf("stat rollback-report.html: %v", statErr)
				}
				if !info.IsDir() {
					t.Error("rollback-report.html = regular file, want it to remain the pre-seeded directory (failed write, not silently replaced)")
				}
			})
		})
	}
}

// TestRollbackCommand_InvalidOutputFlagRemainsOrdinaryError guards that
// this PR's scope is limited to write-failure classification: rollback's
// existing --output validation error must stay an ordinary (exit 1) error,
// checked for both `rollback plan` and `rollback assess`.
func TestRollbackCommand_InvalidOutputFlagRemainsOrdinaryError(t *testing.T) {
	for _, cmdName := range []string{"plan", "assess"} {
		t.Run(cmdName, func(t *testing.T) {
			exitCode := 0
			cmd := newRollbackCmd(&exitCode)
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs([]string{cmdName, "--cluster-name", "prod", "--output", "yaml"})
			err := cmd.Execute()
			if err == nil {
				t.Fatalf("rollback %s --output yaml succeeded, want a validation error", cmdName)
			}
			if isInfraFailure(err) {
				t.Errorf("unsupported --output value marked as an infrastructure failure, want an ordinary exit-1 usage error")
			}
			if got := exitCodeForError(err, 0); got != 1 {
				t.Errorf("exitCodeForError = %d, want 1", got)
			}
		})
	}
}

func baseRollbackAssessment() rollback.Assessment {
	assessment := rollback.NewAssessment(rollback.ModePostUpgradeReadiness, time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC))
	assessment.Cluster = rollback.Cluster{
		Name:                  "prod",
		Provider:              "eks",
		CurrentVersion:        "1.35",
		RollbackTargetVersion: "1.34",
	}
	assessment.Eligibility = rollback.Eligibility{Status: rollback.EligibilityEligible, Source: "amazon-eks"}
	assessment.Readiness = rollback.Readiness{Status: rollback.ReadinessReady}
	assessment.Recommendation = rollback.Recommendation{Decision: rollback.RecommendationOperatorDecisionRequired, Confidence: rollback.ConfidenceMedium}
	assessment.Evidence = rollback.Evidence{Complete: true}
	return assessment
}

func targetPaths(targets []rollbackReportTarget) []string {
	out := make([]string, 0, len(targets))
	for _, target := range targets {
		out = append(out, target.path)
	}
	return out
}
