package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
	"github.com/imneeteeshyadav98/kubepreflight/internal/plan"
)

func samplePlanReport(t *testing.T) *plan.PlanReport {
	t.Helper()

	hop1Findings := []findings.Finding{
		{
			RuleID: "PDB-001", Severity: findings.SeverityBlocker, Confidence: findings.TierStaticCertain,
			Message:     `PodDisruptionBudget "default/critical-app" allows zero disruptions`,
			Resources:   []findings.ResourceReference{findings.LiveResource("PodDisruptionBudget", findings.ScopeNamespaced, "default", "critical-app", "uid-1")},
			Remediation: "Scale up replicas.",
			Fingerprint: "fp-pdb-001",
		},
		{
			RuleID: "WH-001", Severity: findings.SeverityWarning, Confidence: findings.TierStaticCertain,
			Message:     `webhook has catch-all scope`,
			Resources:   []findings.ResourceReference{findings.LiveResource("ValidatingWebhookConfiguration", findings.ScopeCluster, "", "guard", "uid-2")},
			Remediation: "Narrow scope.",
			Fingerprint: "fp-wh-001",
		},
	}
	hop1 := findings.NewReport("1.30", "prod-cluster", "eks", time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC), hop1Findings)

	hop2Findings := []findings.Finding{
		{
			RuleID: "API-001", Severity: findings.SeverityBlocker, Confidence: findings.TierStaticCertain,
			Message:     "manifest uses a removed API",
			Resources:   []findings.ResourceReference{findings.ManifestResource("PodDisruptionBudget", findings.ScopeNamespaced, "default", "old-pdb", "manifests/old.yaml")},
			Remediation: "Migrate apiVersion.",
			Fingerprint: "fp-api-001-predicted",
		},
	}
	hop2Report := findings.NewReport("1.31", "prod-cluster", "eks", time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC), hop2Findings)

	hops := []plan.Hop{
		{Index: 1, From: "1.29", To: "1.30"},
		{Index: 2, From: "1.30", To: "1.31"},
		{Index: 3, From: "1.31", To: "1.32"},
	}

	p, err := plan.BuildPlan("prod-cluster", "eks", "1.29", "eks-describe-cluster", "1.32", hops, hop1,
		func(hop plan.Hop) (plan.HopReport, error) {
			if hop.To == "1.31" {
				return plan.HopReport{Hop: hop, Status: plan.HopStatusPredicted, Report: hop2Report}, nil
			}
			return plan.HopReport{
				Hop:    hop,
				Status: plan.HopStatusPredicted,
				CarryForward: []plan.CarryForwardNote{
					{RuleID: "NODE-001", Reason: "nodes may be replaced before this hop is reached", RecommendedCommand: "kubepreflight scan --target-version " + hop.To},
					{RuleID: "PDB-001", Reason: "PDBs may be fixed before this hop is reached", RecommendedCommand: "kubepreflight scan --target-version " + hop.To},
				},
			}, nil
		}, time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	return p
}

func TestWritePlanCompactSummary_ContainsExpectedContent(t *testing.T) {
	p := samplePlanReport(t)
	var buf bytes.Buffer
	if err := WritePlanCompactSummary(p, &buf); err != nil {
		t.Fatalf("WritePlanCompactSummary: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"cluster: prod-cluster",
		"current: 1.29",
		"target: 1.32",
		"provider: eks",
		"Next hop (1.29 -> 1.30): EXACT",
		"Result: BLOCKED",
		"1 blocker(s), 1 warning(s), 0 info(s)",
		"Path:",
		"1. 1.29 -> 1.30",
		"EXACT",
		"2. 1.30 -> 1.31",
		"PREDICTED",
		"1 blocker(s), 0 warning(s) predicted",
		"3. 1.31 -> 1.32",
		"2 check(s) require a rescan",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("compact plan summary missing %q\n--- full output ---\n%s", want, out)
		}
	}
}

func TestWritePlanCompactSummary_OmitsFindingDetail(t *testing.T) {
	p := samplePlanReport(t)
	var buf bytes.Buffer
	if err := WritePlanCompactSummary(p, &buf); err != nil {
		t.Fatalf("WritePlanCompactSummary: %v", err)
	}
	out := buf.String()

	for _, unwanted := range []string{
		"Evidence:",
		"Remediation:",
		"Scale up replicas",
		"Narrow scope",
		"Migrate apiVersion",
	} {
		if strings.Contains(out, unwanted) {
			t.Errorf("compact plan summary contains %q, want finding detail omitted entirely\n--- full output ---\n%s", unwanted, out)
		}
	}
}
