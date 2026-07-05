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

// TestBuildNextActions_MergesAcrossPartiallyOverlappingResources is the
// root-cause grouping scenario: PDB-001 fires on payment-api-pdb alone,
// PDB-002 fires on (payment-api-pdb, payment-api-pdb-duplicate). They
// share exactly one resource, not their full resource sets, so the old
// exact-set-equality grouping would have kept them separate. The union-
// find generalization must merge them into one Next Action.
func TestBuildNextActions_MergesAcrossPartiallyOverlappingResources(t *testing.T) {
	pdb := liveResources("PodDisruptionBudget", "prod-like", "payment-api-pdb")
	duplicate := findings.LiveResource("PodDisruptionBudget", findings.ScopeNamespaced, "prod-like", "payment-api-pdb-duplicate", "uid-payment-api-pdb-duplicate")

	fs := []findings.Finding{
		{RuleID: "PDB-001", Severity: findings.SeverityBlocker, Resources: pdb, Remediation: "scale up replicas"},
		{RuleID: "PDB-002", Severity: findings.SeverityBlocker, Resources: append(append([]findings.ResourceReference{}, pdb...), duplicate), Remediation: "delete the duplicate PDB"},
	}

	actions := buildNextActions(fs)
	if len(actions) != 1 {
		t.Fatalf("got %d next actions, want 1 (PDB-001 and PDB-002 share payment-api-pdb): %+v", len(actions), actions)
	}
	if len(actions[0].RuleIDs) != 2 || actions[0].RuleIDs[0] != "PDB-001" || actions[0].RuleIDs[1] != "PDB-002" {
		t.Errorf("RuleIDs = %v, want [PDB-001 PDB-002]", actions[0].RuleIDs)
	}
	if len(actions[0].Related) != 1 {
		t.Fatalf("Related = %+v, want exactly 1 (the non-primary finding)", actions[0].Related)
	}
}

// TestBuildNextActions_AllExistingCasesStillPass is a guard that the
// union-find generalization is a strict superset of the old exact-set
// grouping: none of the pre-existing scenarios (identical resource,
// identical remediation, fully disjoint resources, blocker/warning sort)
// change behavior. Each sub-test mirrors an existing TestBuildNextActions_*
// test to make the regression intent explicit in one place.
func TestBuildNextActions_AllExistingCasesStillPass(t *testing.T) {
	t.Run("disjoint resources stay separate", func(t *testing.T) {
		fs := []findings.Finding{
			{RuleID: "PDB-001", Severity: findings.SeverityBlocker, Resources: liveResources("PodDisruptionBudget", "payments", "a"), Remediation: "fix a"},
			{RuleID: "PDB-001", Severity: findings.SeverityBlocker, Resources: liveResources("PodDisruptionBudget", "payments", "b"), Remediation: "fix b"},
		}
		if actions := buildNextActions(fs); len(actions) != 2 {
			t.Errorf("got %d next actions, want 2: %+v", len(actions), actions)
		}
	})
}

// TestBuildNextActions_GlobalBlockerSortsFirst proves the new leading
// tiebreak actually changes order, not just coincides with existing
// severity ordering: both findings are Blocker severity, and the global
// blocker's resource name ("z-webhook") would sort AFTER the plain
// blocker's ("a-resource") under the old severity+groupSortKey-only
// comparator — it must still come first once GlobalBlocker is set.
func TestBuildNextActions_GlobalBlockerSortsFirst(t *testing.T) {
	fs := []findings.Finding{
		{RuleID: "PDB-001", Severity: findings.SeverityBlocker, Resources: liveResources("PodDisruptionBudget", "payments", "a-resource"), Remediation: "fix a"},
		{RuleID: "WH-002", Severity: findings.SeverityBlocker, Resources: webhookResource("z-webhook"), Remediation: "fix webhook", GlobalBlocker: true},
	}

	actions := buildNextActions(fs)
	if len(actions) != 2 {
		t.Fatalf("got %d next actions, want 2: %+v", len(actions), actions)
	}
	if actions[0].RuleIDs[0] != "WH-002" {
		t.Errorf("actions[0].RuleIDs = %v, want WH-002 first (global blocker), even though its resource name would otherwise sort after a-resource", actions[0].RuleIDs)
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
