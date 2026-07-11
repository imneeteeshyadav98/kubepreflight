package plan

import (
	"testing"
	"time"

	"kubepreflight/internal/findings"
)

func TestBuildActionPlanBlocksUpgradeWhenCriticalFindingsExist(t *testing.T) {
	r := findings.NewReport("1.30", "prod", "eks", time.Now(), []findings.Finding{
		{
			RuleID:      "WH-002",
			Severity:    findings.SeverityBlocker,
			Confidence:  findings.TierObserved,
			Message:     "fail-closed webhook has no endpoints",
			Resources:   []findings.ResourceReference{findings.LiveResource("ValidatingWebhookConfiguration", findings.ScopeCluster, "", "guard", "uid-1")},
			Fingerprint: "fp-wh-002",
		},
	})

	actionPlan := BuildActionPlan(r, time.Date(2026, 7, 9, 1, 2, 3, 0, time.UTC))

	if actionPlan.SchemaVersion != ActionPlanSchemaVersion {
		t.Fatalf("SchemaVersion = %q, want %q", actionPlan.SchemaVersion, ActionPlanSchemaVersion)
	}
	if actionPlan.Verdict != "BLOCKED" {
		t.Fatalf("Verdict = %q, want BLOCKED", actionPlan.Verdict)
	}
	if len(actionPlan.Phases) != 4 {
		t.Fatalf("len(Phases) = %d, want 4", len(actionPlan.Phases))
	}

	phase1 := actionPlan.Phases[0]
	if len(phase1.Actions) != 1 {
		t.Fatalf("phase 1 actions = %d, want 1: %+v", len(phase1.Actions), phase1.Actions)
	}
	if phase1.Actions[0].ID != "fix-fail-closed-webhooks" || phase1.Actions[0].Status != ActionStatusRequired {
		t.Fatalf("phase 1 action = %+v, want required webhook action", phase1.Actions[0])
	}
	if got := phase1.Actions[0].SourceRuleIDs; len(got) != 1 || got[0] != "WH-002" {
		t.Fatalf("phase 1 source rules = %+v, want [WH-002]", got)
	}

	for _, action := range actionPlan.Phases[2].Actions {
		if action.Status != ActionStatusBlocked {
			t.Errorf("phase 3 action %s status = %q, want blocked", action.ID, action.Status)
		}
		if action.Reason != "Blocked until critical upgrade blockers are resolved." {
			t.Errorf("phase 3 action %s reason = %q", action.ID, action.Reason)
		}
	}
}

func TestBuildActionPlanAllowsUpgradeWhenNoCriticalBlockers(t *testing.T) {
	r := findings.NewReport("1.30", "prod", "eks", time.Now(), nil)

	actionPlan := BuildActionPlan(r, time.Date(2026, 7, 9, 1, 2, 3, 0, time.UTC))

	if actionPlan.Verdict != "READY" {
		t.Fatalf("Verdict = %q, want READY", actionPlan.Verdict)
	}
	if len(actionPlan.Phases) != 4 {
		t.Fatalf("len(Phases) = %d, want 4", len(actionPlan.Phases))
	}
	if len(actionPlan.Phases[0].Actions) != 0 {
		t.Fatalf("phase 1 actions = %+v, want no critical blocker actions", actionPlan.Phases[0].Actions)
	}
	for _, action := range actionPlan.Phases[2].Actions {
		if action.Status != ActionStatusReady {
			t.Errorf("phase 3 action %s status = %q, want ready", action.ID, action.Status)
		}
		if action.Reason != "" {
			t.Errorf("phase 3 action %s reason = %q, want empty", action.ID, action.Reason)
		}
	}
}

func TestBuildActionPlanIncludesDeprecatedAPIWarningWithoutBlockingUpgrade(t *testing.T) {
	r := findings.NewReport("1.24", "prod", "", time.Now(), []findings.Finding{{
		RuleID:      "API-002",
		Severity:    findings.SeverityWarning,
		Confidence:  findings.TierStaticCertain,
		Message:     "deprecated API still served",
		Resources:   []findings.ResourceReference{findings.ManifestResource("HorizontalPodAutoscaler", findings.ScopeNamespaced, "payments", "payments-hpa", "hpa.yaml")},
		Evidence:    []string{"apiVersion: autoscaling/v2beta2", "removed in: Kubernetes 1.26"},
		Remediation: "Migrate to autoscaling/v2.",
		Fingerprint: "fp-api-002",
	}})

	actionPlan := BuildActionPlan(r, time.Date(2026, 7, 9, 1, 2, 3, 0, time.UTC))
	apiAction := findAction(actionPlan, "fix-api-compatibility")
	if apiAction == nil {
		t.Fatal("fix-api-compatibility action missing")
	}
	if apiAction.Required || apiAction.Status != ActionStatusRecommended {
		t.Fatalf("API-002 warning action = %+v, want non-required recommended action", *apiAction)
	}
	for _, action := range actionPlan.Phases[2].Actions {
		if action.Status != ActionStatusReady {
			t.Errorf("phase 3 action %s status = %q, want ready with API-002 warning only", action.ID, action.Status)
		}
	}
}

func TestBuildActionPlanMapsCriticalFindingsToPhaseOneActions(t *testing.T) {
	r := findings.NewReport("1.30", "prod", "eks", time.Now(), []findings.Finding{
		{
			RuleID:      "API-001",
			Severity:    findings.SeverityBlocker,
			Confidence:  findings.TierStaticCertain,
			Message:     "manifest uses a removed API",
			Resources:   []findings.ResourceReference{findings.ManifestResource("PodDisruptionBudget", findings.ScopeNamespaced, "default", "old-pdb", "manifests/pdb.yaml")},
			Fingerprint: "fp-api-001",
		},
		{
			RuleID:      "WH-001",
			Severity:    findings.SeverityWarning,
			Confidence:  findings.TierStaticCertain,
			Message:     "webhook catches too much traffic",
			Resources:   []findings.ResourceReference{findings.LiveResource("ValidatingWebhookConfiguration", findings.ScopeCluster, "", "guard", "uid-1")},
			Fingerprint: "fp-wh-001",
		},
		{
			RuleID:      "PDB-001",
			Severity:    findings.SeverityBlocker,
			Confidence:  findings.TierObserved,
			Message:     "pdb allows zero disruptions",
			Resources:   []findings.ResourceReference{findings.LiveResource("PodDisruptionBudget", findings.ScopeNamespaced, "default", "api", "uid-2")},
			Fingerprint: "fp-pdb-001",
		},
	})

	actionPlan := BuildActionPlan(r, time.Date(2026, 7, 9, 1, 2, 3, 0, time.UTC))

	got := map[string][]string{}
	for _, action := range actionPlan.Phases[0].Actions {
		got[action.ID] = action.SourceRuleIDs
	}
	for actionID, wantRules := range map[string][]string{
		"fix-api-compatibility":    {"API-001"},
		"fix-fail-closed-webhooks": {"WH-001"},
		"resolve-disruption-risk":  {"PDB-001"},
	} {
		gotRules, ok := got[actionID]
		if !ok {
			t.Fatalf("missing phase 1 action %s in %+v", actionID, actionPlan.Phases[0].Actions)
		}
		if len(gotRules) != len(wantRules) || gotRules[0] != wantRules[0] {
			t.Fatalf("action %s source rules = %+v, want %+v", actionID, gotRules, wantRules)
		}
	}
}

func TestBuildActionPlanAlwaysIncludesPreparationAndValidation(t *testing.T) {
	actionPlan := BuildActionPlan(nil, time.Date(2026, 7, 9, 1, 2, 3, 0, time.UTC))

	if len(actionPlan.Phases) != 4 {
		t.Fatalf("len(Phases) = %d, want 4", len(actionPlan.Phases))
	}
	if len(actionPlan.Phases[1].Actions) != 6 {
		t.Fatalf("phase 2 actions = %d, want 6", len(actionPlan.Phases[1].Actions))
	}
	if len(actionPlan.Phases[3].Actions) != 6 {
		t.Fatalf("phase 4 actions = %d, want 6", len(actionPlan.Phases[3].Actions))
	}
}

func TestBuildActionPlanIncludesWorkloadWarningWithoutBlockingUpgrade(t *testing.T) {
	r := findings.NewReport("1.30", "prod", "eks", time.Now(), []findings.Finding{
		{
			RuleID:      "WORKLOAD-001",
			Severity:    findings.SeverityWarning,
			Confidence:  findings.TierObserved,
			Message:     "workload has unhealthy pods before upgrade",
			Resources:   []findings.ResourceReference{findings.LiveResource("Pod", findings.ScopeNamespaced, "default", "api-abc", "uid-pod")},
			Fingerprint: "fp-workload-001",
		},
	})

	actionPlan := BuildActionPlan(r, time.Date(2026, 7, 9, 1, 2, 3, 0, time.UTC))

	var workloadAction *PlanAction
	for i := range actionPlan.Phases[0].Actions {
		if actionPlan.Phases[0].Actions[i].ID == "resolve-unhealthy-workloads" {
			workloadAction = &actionPlan.Phases[0].Actions[i]
			break
		}
	}
	if workloadAction == nil {
		t.Fatalf("phase 1 missing resolve-unhealthy-workloads action: %+v", actionPlan.Phases[0].Actions)
	}
	if workloadAction.Required || workloadAction.Status != ActionStatusRecommended {
		t.Fatalf("workload action = %+v, want non-required recommended action for warning finding", *workloadAction)
	}
	if got := workloadAction.SourceRuleIDs; len(got) != 1 || got[0] != "WORKLOAD-001" {
		t.Fatalf("workload action source rules = %+v, want [WORKLOAD-001]", got)
	}
	for _, action := range actionPlan.Phases[2].Actions {
		if action.Status != ActionStatusReady {
			t.Errorf("phase 3 action %s status = %q, want ready when only WORKLOAD-001 warning exists", action.ID, action.Status)
		}
	}
}

func TestBuildActionPlanBlocksUpgradeWhenWorkloadFindingBecomesBlocker(t *testing.T) {
	r := findings.NewReport("1.30", "prod", "eks", time.Now(), []findings.Finding{
		{
			RuleID:      "WORKLOAD-001",
			Severity:    findings.SeverityBlocker,
			Confidence:  findings.TierObserved,
			Message:     "critical workload has unhealthy pods before upgrade",
			Resources:   []findings.ResourceReference{findings.LiveResource("Pod", findings.ScopeNamespaced, "default", "api-abc", "uid-pod")},
			Fingerprint: "fp-workload-001",
		},
	})

	actionPlan := BuildActionPlan(r, time.Date(2026, 7, 9, 1, 2, 3, 0, time.UTC))

	for _, action := range actionPlan.Phases[2].Actions {
		if action.Status != ActionStatusBlocked {
			t.Errorf("phase 3 action %s status = %q, want blocked when WORKLOAD-001 is blocker", action.ID, action.Status)
		}
	}
}

func TestBuildActionPlanIncludesDeprecatedMasterLabelWarningWithoutBlockingUpgrade(t *testing.T) {
	r := findings.NewReport("1.30", "prod", "eks", time.Now(), []findings.Finding{
		{
			RuleID:      "NODE-003",
			Severity:    findings.SeverityWarning,
			Confidence:  findings.TierStaticCertain,
			Message:     "workload uses deprecated master node label",
			Resources:   []findings.ResourceReference{findings.LiveResource("Deployment", findings.ScopeNamespaced, "default", "legacy-pinned", "uid-deploy")},
			Fingerprint: "fp-node-003",
		},
	})

	actionPlan := BuildActionPlan(r, time.Date(2026, 7, 9, 1, 2, 3, 0, time.UTC))

	var nodeAction *PlanAction
	for i := range actionPlan.Phases[0].Actions {
		if actionPlan.Phases[0].Actions[i].ID == "replace-deprecated-master-node-label" {
			nodeAction = &actionPlan.Phases[0].Actions[i]
			break
		}
	}
	if nodeAction == nil {
		t.Fatalf("phase 1 missing replace-deprecated-master-node-label action: %+v", actionPlan.Phases[0].Actions)
	}
	if nodeAction.Required || nodeAction.Status != ActionStatusRecommended {
		t.Fatalf("NODE-003 warning action = %+v, want non-required recommended action", *nodeAction)
	}
	for _, action := range actionPlan.Phases[2].Actions {
		if action.Status != ActionStatusReady {
			t.Errorf("phase 3 action %s status = %q, want ready when only NODE-003 warning exists", action.ID, action.Status)
		}
	}
}

func TestBuildActionPlanBlocksUpgradeWhenDeprecatedMasterLabelAffectsCriticalInfra(t *testing.T) {
	r := findings.NewReport("1.30", "prod", "eks", time.Now(), []findings.Finding{
		{
			RuleID:        "NODE-003",
			Severity:      findings.SeverityBlocker,
			Confidence:    findings.TierStaticCertain,
			Message:       "critical workload uses deprecated master node label",
			Resources:     []findings.ResourceReference{findings.LiveResource("DaemonSet", findings.ScopeNamespaced, "kube-system", "cni", "uid-ds")},
			Fingerprint:   "fp-node-003-critical",
			CriticalInfra: true,
		},
	})

	actionPlan := BuildActionPlan(r, time.Date(2026, 7, 9, 1, 2, 3, 0, time.UTC))

	for _, action := range actionPlan.Phases[2].Actions {
		if action.Status != ActionStatusBlocked {
			t.Errorf("phase 3 action %s status = %q, want blocked when critical NODE-003 exists", action.ID, action.Status)
		}
	}
}

func findAction(actionPlan *UpgradeActionPlan, id string) *PlanAction {
	for phaseIndex := range actionPlan.Phases {
		for actionIndex := range actionPlan.Phases[phaseIndex].Actions {
			if actionPlan.Phases[phaseIndex].Actions[actionIndex].ID == id {
				return &actionPlan.Phases[phaseIndex].Actions[actionIndex]
			}
		}
	}
	return nil
}
