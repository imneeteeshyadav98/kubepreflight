package report

import (
	"testing"

	"kubepreflight/internal/findings"
)

func webhookResource(name string) []findings.ResourceReference {
	return []findings.ResourceReference{findings.LiveResource("ValidatingWebhookConfiguration", findings.ScopeCluster, "", name, "uid-"+name)}
}

func liveResources(kind, namespace, name string) []findings.ResourceReference {
	scope := findings.ScopeCluster
	if namespace != "" {
		scope = findings.ScopeNamespaced
	}
	return []findings.ResourceReference{findings.LiveResource(kind, scope, namespace, name, "uid-"+name)}
}

func TestBuildNextActions_MergesFindingsOnSameResource(t *testing.T) {
	// The exact WH-001/WH-002 scenario: two rules fire on the same
	// resource with different severities and different remediation text.
	fs := []findings.Finding{
		{RuleID: "WH-001", Severity: findings.SeverityWarning, Resources: webhookResource("payments-guard"), Remediation: "Narrow the webhook's scope."},
		{RuleID: "WH-002", Severity: findings.SeverityBlocker, Resources: webhookResource("payments-guard"), Remediation: "Restore backend health via kubectl patch."},
	}

	actions := buildNextActions(fs)
	if len(actions) != 1 {
		t.Fatalf("got %d next actions, want 1 (same resource must merge): %+v", len(actions), actions)
	}

	a := actions[0]
	if a.Severity != findings.SeverityBlocker {
		t.Errorf("Severity = %q, want Blocker (highest severity in group)", a.Severity)
	}
	if a.Primary.RuleID != "WH-002" {
		t.Errorf("Primary.RuleID = %q, want WH-002 (higher severity wins)", a.Primary.RuleID)
	}
	if len(a.RuleIDs) != 2 || a.RuleIDs[0] != "WH-001" || a.RuleIDs[1] != "WH-002" {
		t.Errorf("RuleIDs = %v, want [WH-001 WH-002] (sorted)", a.RuleIDs)
	}
	if len(a.Related) != 1 || a.Related[0].RuleID != "WH-001" {
		t.Errorf("Related = %+v, want exactly WH-001 (the non-primary finding)", a.Related)
	}
}

func TestBuildNextActions_IdenticalRemediationNotDuplicatedInRelated(t *testing.T) {
	fs := []findings.Finding{
		{RuleID: "AAA-001", Severity: findings.SeverityBlocker, Resources: webhookResource("x"), Remediation: "Do the fix."},
		{RuleID: "BBB-001", Severity: findings.SeverityBlocker, Resources: webhookResource("x"), Remediation: "Do the fix."},
	}

	actions := buildNextActions(fs)
	if len(actions) != 1 {
		t.Fatalf("got %d next actions, want 1", len(actions))
	}
	if len(actions[0].Related) != 0 {
		t.Errorf("Related = %+v, want empty (identical remediation text must not repeat as a pointless pointer)", actions[0].Related)
	}
}

func TestBuildNextActions_DifferentResourcesStaySeparate(t *testing.T) {
	fs := []findings.Finding{
		{RuleID: "PDB-001", Severity: findings.SeverityBlocker, Resources: liveResources("PodDisruptionBudget", "payments", "a"), Remediation: "fix a"},
		{RuleID: "PDB-001", Severity: findings.SeverityBlocker, Resources: liveResources("PodDisruptionBudget", "payments", "b"), Remediation: "fix b"},
	}

	actions := buildNextActions(fs)
	if len(actions) != 2 {
		t.Fatalf("got %d next actions, want 2 (distinct resources must not merge): %+v", len(actions), actions)
	}
}

func TestBuildNextActions_SortedBlockersBeforeWarnings(t *testing.T) {
	fs := []findings.Finding{
		{RuleID: "WH-001", Severity: findings.SeverityWarning, Resources: liveResources("ConfigMap", "", "z-resource"), Remediation: "r1"},
		{RuleID: "PDB-001", Severity: findings.SeverityBlocker, Resources: liveResources("PodDisruptionBudget", "", "a-resource"), Remediation: "r2"},
	}

	actions := buildNextActions(fs)
	if len(actions) != 2 {
		t.Fatalf("got %d next actions, want 2", len(actions))
	}
	if actions[0].Severity != findings.SeverityBlocker {
		t.Errorf("actions[0].Severity = %q, want Blocker to sort first regardless of resource name", actions[0].Severity)
	}
	if actions[1].Severity != findings.SeverityWarning {
		t.Errorf("actions[1].Severity = %q, want Warning last", actions[1].Severity)
	}
}

func TestBuildNextActions_EmptyInput(t *testing.T) {
	if got := buildNextActions(nil); got != nil {
		t.Errorf("buildNextActions(nil) = %v, want nil", got)
	}
}

func TestFilterAndSort(t *testing.T) {
	fs := []findings.Finding{
		{RuleID: "WH-002", Severity: findings.SeverityBlocker, Resources: liveResources("Test", "", "b")},
		{RuleID: "WH-001", Severity: findings.SeverityWarning, Resources: liveResources("Test", "", "a")},
		{RuleID: "WH-001", Severity: findings.SeverityBlocker, Resources: liveResources("Test", "", "b")},
		{RuleID: "WH-001", Severity: findings.SeverityBlocker, Resources: liveResources("Test", "", "a")},
	}

	blockers := filterAndSort(fs, findings.SeverityBlocker)
	if len(blockers) != 3 {
		t.Fatalf("got %d blockers, want 3", len(blockers))
	}
	// Sorted by RuleID then Resource.Name: WH-001/a, WH-001/b, WH-002/b.
	if blockers[0].Resources[0].Name != "a" || blockers[1].Resources[0].Name != "b" || blockers[2].RuleID != "WH-002" {
		t.Errorf("blockers not sorted as expected: %+v", blockers)
	}

	warnings := filterAndSort(fs, findings.SeverityWarning)
	if len(warnings) != 1 {
		t.Fatalf("got %d warnings, want 1", len(warnings))
	}
}
