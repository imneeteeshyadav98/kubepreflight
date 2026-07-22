package rules

import (
	"testing"
	"time"

	awscol "github.com/imneeteeshyadav98/kubepreflight/internal/collectors/aws"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

func TestNET002_Positive_MissingSecurityGroup(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{
		NetworkPreflightIssues: []awscol.NetworkPreflightIssue{
			{Kind: "SecurityGroup", ID: "sg-deleted"},
		},
	}, UpgradeContext: findings.UpgradeContextControlPlaneOnly}

	fs, err := (NET002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}

	f := fs[0]
	if f.RuleID != "NET-002" {
		t.Errorf("RuleID = %q, want NET-002", f.RuleID)
	}
	if f.Severity != findings.SeverityBlocker {
		t.Errorf("Severity = %q, want Blocker", f.Severity)
	}
	if f.Confidence != findings.TierStaticCertain {
		t.Errorf("Confidence = %q, want STATIC_CERTAIN", f.Confidence)
	}
	if f.Resources[0].Name != "sg-deleted" {
		t.Errorf("resource name = %q, want sg-deleted", f.Resources[0].Name)
	}
}

func TestNET002_Positive_MissingVpc(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{
		NetworkPreflightIssues: []awscol.NetworkPreflightIssue{
			{Kind: "Vpc", ID: "vpc-deleted"},
		},
	}, UpgradeContext: findings.UpgradeContextControlPlaneOnly}

	fs, err := (NET002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
	if fs[0].Resources[0].Name != "vpc-deleted" {
		t.Errorf("resource name = %q, want vpc-deleted", fs[0].Resources[0].Name)
	}
}

func TestNET002_Negative_NoIssuesNoFindings(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{}}

	fs, err := (NET002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 (no network preflight issues): %+v", len(fs), fs)
	}
}

func TestNET002_Negative_NilAWSSnapshotNoFindingsNoError(t *testing.T) {
	sc := &ScanContext{AWS: nil}
	fs, err := (NET002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate must not error when AWS enrichment was skipped: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 when sc.AWS is nil: %+v", len(fs), fs)
	}
}

func TestNET002_ContextMatrix(t *testing.T) {
	cases := []struct {
		name             string
		ctx              findings.UpgradeContext
		wantSeverity     findings.Severity
		wantGate         findings.UpgradeGate
		wantBlockers     int
		wantWarnings     int
		wantOperatorDecs int
		wantCanContinue  bool
		wantExitCode     int
		wantResult       string
		wantPriority     string
	}{
		{
			name: "unspecified", ctx: findings.UpgradeContextUnspecified,
			wantSeverity: findings.SeverityWarning, wantGate: findings.UpgradeGateOperatorDecision,
			wantWarnings: 1, wantOperatorDecs: 1, wantCanContinue: false, wantExitCode: 1, wantResult: "PASSED_WITH_WARNINGS", wantPriority: "P2",
		},
		{
			name: "audit only", ctx: findings.UpgradeContextAuditOnly,
			wantSeverity: findings.SeverityWarning, wantGate: findings.UpgradeGateAllow,
			wantWarnings: 1, wantCanContinue: true, wantExitCode: 1, wantResult: "PASSED_WITH_WARNINGS", wantPriority: "P2",
		},
		{
			name: "control plane only", ctx: findings.UpgradeContextControlPlaneOnly,
			wantSeverity: findings.SeverityBlocker, wantGate: findings.UpgradeGateBlock,
			wantBlockers: 1, wantCanContinue: false, wantExitCode: 2, wantResult: "BLOCKED", wantPriority: "P2",
		},
		{
			name: "worker rollout", ctx: findings.UpgradeContextWorkerRollout,
			wantSeverity: findings.SeverityWarning, wantGate: findings.UpgradeGateOperatorDecision,
			wantWarnings: 1, wantOperatorDecs: 1, wantCanContinue: false, wantExitCode: 1, wantResult: "PASSED_WITH_WARNINGS", wantPriority: "P2",
		},
		{
			name: "full platform upgrade", ctx: findings.UpgradeContextFullPlatformUpgrade,
			wantSeverity: findings.SeverityBlocker, wantGate: findings.UpgradeGateBlock,
			wantBlockers: 1, wantCanContinue: false, wantExitCode: 2, wantResult: "BLOCKED", wantPriority: "P2",
		},
		{
			name: "workload restart", ctx: findings.UpgradeContextWorkloadRestart,
			wantSeverity: findings.SeverityWarning, wantGate: findings.UpgradeGateOperatorDecision,
			wantWarnings: 1, wantOperatorDecs: 1, wantCanContinue: false, wantExitCode: 1, wantResult: "PASSED_WITH_WARNINGS", wantPriority: "P2",
		},
	}

	var stableFingerprint string
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sc := &ScanContext{AWS: &awscol.Snapshot{
				NetworkPreflightIssues: []awscol.NetworkPreflightIssue{{Kind: "SecurityGroup", ID: "sg-deleted"}},
			}, UpgradeContext: tc.ctx}
			fs, err := (NET002{}).Evaluate(sc, "1.34")
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if len(fs) != 1 {
				t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
			}
			raw := fs[0]
			if raw.RuleID != "NET-002" {
				t.Fatalf("RuleID = %q, want NET-002", raw.RuleID)
			}
			if raw.Severity != tc.wantSeverity || raw.EffectiveUpgradeGate() != tc.wantGate {
				t.Fatalf("raw severity/gate = %s/%s, want %s/%s", raw.Severity, raw.EffectiveUpgradeGate(), tc.wantSeverity, tc.wantGate)
			}
			if !hasImpactScope(raw, findings.ImpactScopeControlPlaneUpgrade) {
				t.Fatalf("ImpactScopes = %v, want %s", raw.ImpactScopes, findings.ImpactScopeControlPlaneUpgrade)
			}
			if stableFingerprint == "" {
				stableFingerprint = raw.Fingerprint
			} else if raw.Fingerprint != stableFingerprint {
				t.Fatalf("Fingerprint = %q, want stable %q", raw.Fingerprint, stableFingerprint)
			}

			rpt := findings.NewReportWithUpgradeContext("1.34", "test", "eks", tc.ctx, time.Unix(0, 0).UTC(), fs)
			if rpt.Summary.Blockers != tc.wantBlockers || rpt.Summary.Warnings != tc.wantWarnings || rpt.Summary.OperatorDecisions != tc.wantOperatorDecs {
				t.Fatalf("summary = %+v, want blockers=%d warnings=%d operatorDecisions=%d", rpt.Summary, tc.wantBlockers, tc.wantWarnings, tc.wantOperatorDecs)
			}
			got := rpt.Findings[0]
			if got.CanUpgradeContinue != tc.wantCanContinue {
				t.Fatalf("CanUpgradeContinue = %t, want %t", got.CanUpgradeContinue, tc.wantCanContinue)
			}
			if got.Priority != tc.wantPriority {
				t.Fatalf("Priority = %q, want %q", got.Priority, tc.wantPriority)
			}
			if rpt.ExitCode() != tc.wantExitCode || rpt.Result() != tc.wantResult {
				t.Fatalf("result/exit = %s/%d, want %s/%d", rpt.Result(), rpt.ExitCode(), tc.wantResult, tc.wantExitCode)
			}
		})
	}
}
