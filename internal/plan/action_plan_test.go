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
