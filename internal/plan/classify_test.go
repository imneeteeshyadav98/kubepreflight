package plan

import (
	"testing"

	"kubepreflight/internal/rules"
)

func TestPolicyFor_KnownRules(t *testing.T) {
	tests := []struct {
		ruleID string
		want   RuleProjectionPolicy
	}{
		{"API-001", ProjectFromManifests},
		{"API-002", ProjectFromFreshAWSQuery},
		{"ADDON-001", ProjectFromFreshAWSQuery},
		{"EKS-NG-001", CarryForwardOnly},
		{"EKS-NG-002", CarryForwardOnly},
		{"EKS-NG-003", CarryForwardOnly},
		{"EKS-NG-004", CarryForwardOnly},
		{"WH-001", CarryForwardOnly},
		{"WH-002", CarryForwardOnly},
		{"PDB-001", CarryForwardOnly},
		{"PDB-002", CarryForwardOnly},
		{"NODE-001", CarryForwardOnly},
		{"NODE-002", CarryForwardOnly},
		{"NET-002", CarryForwardOnly},
		{"COREDNS-001", CarryForwardOnly},
	}
	for _, tc := range tests {
		t.Run(tc.ruleID, func(t *testing.T) {
			if got := PolicyFor(tc.ruleID); got != tc.want {
				t.Errorf("PolicyFor(%q) = %v, want %v", tc.ruleID, got, tc.want)
			}
		})
	}
}

func TestPolicyFor_UnknownRuleDefaultsToCarryForward(t *testing.T) {
	if got := PolicyFor("UNKNOWN-999"); got != CarryForwardOnly {
		t.Errorf("PolicyFor(unknown) = %v, want CarryForwardOnly (fail-safe default)", got)
	}
}

// TestRulePolicy_CoversEveryRegisteredRule guards against a newly
// registered rule (rules.NewDefaultRegistry) silently falling through to
// PolicyFor's CarryForwardOnly default without an explicit, reviewed entry
// in RulePolicy — a rule added to the scanner should never automatically
// gain (or lose) confidence in a multi-hop plan's future stages without
// that being a deliberate decision made in classify.go.
func TestRulePolicy_CoversEveryRegisteredRule(t *testing.T) {
	registry := rules.NewDefaultRegistry()
	for _, ruleID := range registry.RuleIDs() {
		if _, ok := RulePolicy[ruleID]; !ok {
			t.Errorf("rule %q is registered in rules.NewDefaultRegistry but has no explicit entry in plan.RulePolicy", ruleID)
		}
	}
}
