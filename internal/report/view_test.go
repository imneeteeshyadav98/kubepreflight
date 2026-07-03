package report

import (
	"testing"

	"kubepreflight/internal/findings"
)

func webhookResource(name string) findings.Resource {
	return findings.Resource{Kind: "ValidatingWebhookConfiguration", Name: name, UID: "uid-" + name}
}

func TestBuildNextActions_MergesFindingsOnSameResource(t *testing.T) {
	// The exact WH-001/WH-002 scenario: two rules fire on the same
	// resource with different severities and different remediation text.
	fs := []findings.Finding{
		{RuleID: "WH-001", Severity: findings.SeverityWarning, Resource: webhookResource("payments-guard"), Remediation: "Narrow the webhook's scope."},
		{RuleID: "WH-002", Severity: findings.SeverityBlocker, Resource: webhookResource("payments-guard"), Remediation: "Restore backend health via kubectl patch."},
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
		{RuleID: "AAA-001", Severity: findings.SeverityBlocker, Resource: webhookResource("x"), Remediation: "Do the fix."},
		{RuleID: "BBB-001", Severity: findings.SeverityBlocker, Resource: webhookResource("x"), Remediation: "Do the fix."},
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
		{RuleID: "PDB-001", Severity: findings.SeverityBlocker, Resource: findings.Resource{Kind: "PodDisruptionBudget", Namespace: "payments", Name: "a"}, Remediation: "fix a"},
		{RuleID: "PDB-001", Severity: findings.SeverityBlocker, Resource: findings.Resource{Kind: "PodDisruptionBudget", Namespace: "payments", Name: "b"}, Remediation: "fix b"},
	}

	actions := buildNextActions(fs)
	if len(actions) != 2 {
		t.Fatalf("got %d next actions, want 2 (distinct resources must not merge): %+v", len(actions), actions)
	}
}

func TestBuildNextActions_SortedBlockersBeforeWarnings(t *testing.T) {
	fs := []findings.Finding{
		{RuleID: "WH-001", Severity: findings.SeverityWarning, Resource: findings.Resource{Kind: "ConfigMap", Name: "z-resource"}, Remediation: "r1"},
		{RuleID: "PDB-001", Severity: findings.SeverityBlocker, Resource: findings.Resource{Kind: "PodDisruptionBudget", Name: "a-resource"}, Remediation: "r2"},
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
		{RuleID: "WH-002", Severity: findings.SeverityBlocker, Resource: findings.Resource{Name: "b"}},
		{RuleID: "WH-001", Severity: findings.SeverityWarning, Resource: findings.Resource{Name: "a"}},
		{RuleID: "WH-001", Severity: findings.SeverityBlocker, Resource: findings.Resource{Name: "b"}},
		{RuleID: "WH-001", Severity: findings.SeverityBlocker, Resource: findings.Resource{Name: "a"}},
	}

	blockers := filterAndSort(fs, findings.SeverityBlocker)
	if len(blockers) != 3 {
		t.Fatalf("got %d blockers, want 3", len(blockers))
	}
	// Sorted by RuleID then Resource.Name: WH-001/a, WH-001/b, WH-002/b.
	if blockers[0].Resource.Name != "a" || blockers[1].Resource.Name != "b" || blockers[2].RuleID != "WH-002" {
		t.Errorf("blockers not sorted as expected: %+v", blockers)
	}

	warnings := filterAndSort(fs, findings.SeverityWarning)
	if len(warnings) != 1 {
		t.Fatalf("got %d warnings, want 1", len(warnings))
	}
}
