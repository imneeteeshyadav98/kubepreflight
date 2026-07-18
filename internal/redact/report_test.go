package redact

import (
	"testing"
	"time"

	"kubepreflight/internal/findings"
	"kubepreflight/internal/plan"
	"kubepreflight/internal/rollback"
)

func realFinding() findings.Finding {
	ref := findings.LiveResource("Node", findings.ScopeCluster, "", realHostname, "uid-node-1")
	f := findings.Finding{
		RuleID:      "DRAIN-003",
		Severity:    findings.SeverityWarning,
		Confidence:  findings.TierObserved,
		Message:     "qualifying node(s): " + realHostname,
		Resources:   []findings.ResourceReference{ref},
		Evidence:    []string{"qualifying node(s): " + realHostname},
		Remediation: "Label additional nodes matching " + realHostname,
		RemediationDetail: &findings.RemediationDetail{
			VerifyCommand:  "kubectl get node " + realHostname,
			ExpectedResult: "node " + realHostname + " has the expected labels",
			Changes: []findings.RemediationChange{
				{Field: "nodeSelector", Current: realHostname, Required: "any-labeled-node"},
			},
			SafeFix: &findings.RemediationAction{
				Label:   "Label additional nodes",
				Steps:   []string{"identify a replacement for " + realHostname},
				Command: "kubectl label node " + realHostname + " zone=a",
			},
			BreakGlass: &findings.RemediationAction{
				Label:   "Force delete",
				Command: "kubectl delete node " + realHostname + " --force",
				Risky:   true,
			},
		},
		Fingerprint: findings.FingerprintV2("DRAIN-003", "1.36", "", ref),
	}
	return findings.AssignPriority(f)
}

func realReport() *findings.Report {
	r := findings.NewReport("1.36", realARN, "eks", time.Now().UTC(), []findings.Finding{realFinding()})
	r.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS:        findings.PlaneCoverage{Status: findings.CoveragePartial, Errors: []string{"AccessDenied: User " + realARN + " is not authorized"}},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageSkipped},
	})
	r.EKSCluster = &findings.EKSClusterInfo{ClusterName: "kubepreflight-live-demo", Region: "us-east-1", ARN: realARN}
	r.EKSNodegroups = []findings.EKSNodegroupInfo{
		{
			Name:              "ng-small",
			AutoScalingGroups: []string{"eks-ng-small-" + realARN},
			HealthIssues:      []findings.EKSNodegroupHealthIssue{{Code: "NodeCreationFailure", Message: "failed for " + realARN}},
		},
	}
	r.EKSUpgradeInsights = []findings.EKSUpgradeInsightInfo{
		{
			ID: "insight-1", Name: "Insight", Category: "cat", Status: "PASSING",
			Description:    "resource " + realARN,
			Recommendation: "review " + realHostname,
			AdditionalInfo: map[string]string{"resource": realARN},
		},
	}
	return r
}

func assertNoLeak(t *testing.T, label, s string) {
	t.Helper()
	if s == "" {
		return
	}
	if got := Text(s); got != s {
		t.Errorf("%s still contains a sensitive value after redaction: %q (Text() would still change it to %q)", label, s, got)
	}
}

func TestReport_RedactsEveryReachableField(t *testing.T) {
	r := realReport()
	Report(r)

	assertNoLeak(t, "Report.ClusterContext", r.ClusterContext)
	assertNoLeak(t, "Report.Coverage.AWS.Errors[0]", r.Coverage.AWS.Errors[0])
	assertNoLeak(t, "Report.EKSCluster.ARN", r.EKSCluster.ARN)
	assertNoLeak(t, "EKSNodegroups[0].AutoScalingGroups[0]", r.EKSNodegroups[0].AutoScalingGroups[0])
	assertNoLeak(t, "EKSNodegroups[0].HealthIssues[0].Message", r.EKSNodegroups[0].HealthIssues[0].Message)
	assertNoLeak(t, "EKSUpgradeInsights[0].Description", r.EKSUpgradeInsights[0].Description)
	assertNoLeak(t, "EKSUpgradeInsights[0].Recommendation", r.EKSUpgradeInsights[0].Recommendation)
	assertNoLeak(t, "EKSUpgradeInsights[0].AdditionalInfo[resource]", r.EKSUpgradeInsights[0].AdditionalInfo["resource"])

	f := r.Findings[0]
	assertNoLeak(t, "Finding.Message", f.Message)
	assertNoLeak(t, "Finding.Evidence[0]", f.Evidence[0])
	assertNoLeak(t, "Finding.Remediation", f.Remediation)
	assertNoLeak(t, "Finding.Resources[0].Name", f.Resources[0].Name)
	assertNoLeak(t, "RemediationDetail.VerifyCommand", f.RemediationDetail.VerifyCommand)
	assertNoLeak(t, "RemediationDetail.ExpectedResult", f.RemediationDetail.ExpectedResult)
	assertNoLeak(t, "RemediationDetail.Changes[0].Current", f.RemediationDetail.Changes[0].Current)
	assertNoLeak(t, "RemediationDetail.SafeFix.Command", f.RemediationDetail.SafeFix.Command)
	assertNoLeak(t, "RemediationDetail.SafeFix.Steps[0]", f.RemediationDetail.SafeFix.Steps[0])
	assertNoLeak(t, "RemediationDetail.BreakGlass.Command", f.RemediationDetail.BreakGlass.Command)
}

func TestReport_PreservesDetectorFacts(t *testing.T) {
	r := realReport()
	beforeFingerprint := r.Findings[0].Fingerprint
	beforeSeverity := r.Findings[0].Severity
	beforeRuleID := r.Findings[0].RuleID
	beforeScore := r.UpgradeReadiness.ReadinessScore
	beforeVerdict := r.UpgradeReadiness.Verdict
	beforeExitCode := r.ExitCode()

	Report(r)

	if r.Findings[0].Fingerprint != beforeFingerprint {
		t.Error("Report() changed a finding's Fingerprint — this would break kubepreflight compare's cross-scan matching")
	}
	if r.Findings[0].Severity != beforeSeverity {
		t.Error("Report() changed a finding's Severity")
	}
	if r.Findings[0].RuleID != beforeRuleID {
		t.Error("Report() changed a finding's RuleID")
	}
	if r.UpgradeReadiness.ReadinessScore != beforeScore {
		t.Error("Report() changed the readiness score")
	}
	if r.UpgradeReadiness.Verdict != beforeVerdict {
		t.Error("Report() changed the verdict")
	}
	if r.ExitCode() != beforeExitCode {
		t.Error("Report() changed the exit code")
	}
}

func TestReport_NilSafe(t *testing.T) {
	Report(nil) // must not panic
}

func TestReport_ClusterNameAndRegionNotRedacted(t *testing.T) {
	// Cluster name and region are not treated as sensitive anywhere else in
	// this product (see the real EKS case-study evidence, which redacted
	// only the ARN and node hostname, never the cluster name or region) —
	// this test locks that decision in so it can't regress silently.
	r := realReport()
	Report(r)
	if r.EKSCluster.ClusterName != "kubepreflight-live-demo" {
		t.Errorf("EKSCluster.ClusterName was redacted, want it left alone: %q", r.EKSCluster.ClusterName)
	}
	if r.EKSCluster.Region != "us-east-1" {
		t.Errorf("EKSCluster.Region was redacted, want it left alone: %q", r.EKSCluster.Region)
	}
}

func realAssessment() rollback.Assessment {
	a := rollback.NewAssessment(rollback.ModePostUpgradeReadiness, time.Now().UTC())
	a.Cluster = rollback.Cluster{Name: "kubepreflight-live-demo", Region: "us-east-1", CurrentVersion: "1.36", Provider: "eks"}
	a.Eligibility = rollback.Eligibility{Status: rollback.EligibilityEligible}
	a.Readiness = rollback.Readiness{Status: rollback.ReadinessBlocked, Blockers: 1}
	a.Recommendation = rollback.Recommendation{Decision: rollback.RecommendationDoNotProceed, Confidence: rollback.ConfidenceHigh}
	a.Evidence = rollback.Evidence{Complete: true}
	a.Checks = []rollback.Check{
		{ID: "reverse-compatibility", Title: "API compatibility", Status: rollback.CheckFail, Evidence: []string{"resource: arn=" + realARN + " status=fail"}},
	}
	return a
}

func TestRollbackAssessment_RedactsEvidence(t *testing.T) {
	a := realAssessment()
	RollbackAssessment(&a)
	assertNoLeak(t, "Checks[0].Evidence[0]", a.Checks[0].Evidence[0])
}

func TestRollbackAssessment_PreservesDecisionFacts(t *testing.T) {
	a := realAssessment()
	beforeDecision := a.Recommendation.Decision
	beforeStatus := a.Readiness.Status
	RollbackAssessment(&a)
	if a.Recommendation.Decision != beforeDecision {
		t.Error("RollbackAssessment() changed Recommendation.Decision")
	}
	if a.Readiness.Status != beforeStatus {
		t.Error("RollbackAssessment() changed Readiness.Status")
	}
	if a.Cluster.Name != "kubepreflight-live-demo" {
		t.Errorf("Cluster.Name was redacted, want it left alone: %q", a.Cluster.Name)
	}
}

func TestRollbackAssessment_NilSafe(t *testing.T) {
	RollbackAssessment(nil) // must not panic
}

func TestPlanReport_RedactsHopsAndActionPlan(t *testing.T) {
	hop1 := realReport()
	pr := &plan.PlanReport{
		SchemaVersion:  findings.SchemaVersion,
		ClusterContext: realARN,
		Hops: []plan.HopReport{
			{Status: plan.HopStatusExact, Report: hop1},
		},
		ActionPlan: &plan.UpgradeActionPlan{
			Phases: []plan.ActionPhase{
				{
					Description: "resource " + realARN,
					Gate:        "blocked by " + realHostname,
					Actions: []plan.PlanAction{
						{
							Title:           "Fix " + realHostname,
							Reason:          "affects " + realARN,
							SuccessCriteria: []string{"node " + realHostname + " passes"},
							Commands:        []string{"kubectl drain " + realHostname},
						},
					},
				},
			},
		},
	}

	PlanReport(pr)

	assertNoLeak(t, "PlanReport.ClusterContext", pr.ClusterContext)
	assertNoLeak(t, "Hops[0].Report.ClusterContext", pr.Hops[0].Report.ClusterContext)
	phase := pr.ActionPlan.Phases[0]
	assertNoLeak(t, "ActionPlan.Phases[0].Description", phase.Description)
	assertNoLeak(t, "ActionPlan.Phases[0].Gate", phase.Gate)
	action := phase.Actions[0]
	assertNoLeak(t, "ActionPlan action Title", action.Title)
	assertNoLeak(t, "ActionPlan action Reason", action.Reason)
	assertNoLeak(t, "ActionPlan action SuccessCriteria[0]", action.SuccessCriteria[0])
	assertNoLeak(t, "ActionPlan action Commands[0]", action.Commands[0])
}

func TestPlanReport_NilSafe(t *testing.T) {
	PlanReport(nil)                // must not panic
	PlanReport(&plan.PlanReport{}) // no hops, no action plan — must not panic
}
