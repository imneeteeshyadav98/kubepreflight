package findings

import (
	"testing"
	"time"
)

func TestAssignPriority_MappingByRuleID(t *testing.T) {
	cases := []struct {
		ruleID string
		want   Priority
	}{
		{"API-001", PriorityP2},
		{"EKS-INSIGHT-001", PriorityP2},
		{"CRD-001", PriorityP2},
		{"CRD-002", PriorityP2},
		{"APISERVICE-001", PriorityP2},
		{"NET-002", PriorityP2},
		{"NODE-002", PriorityP2},

		{"PDB-001", PriorityP3},
		{"PDB-002", PriorityP3},
		{"NODE-001", PriorityP3},
		{"EKS-NG-002", PriorityP3},

		{"WH-001", PriorityP4},
		{"WH-002", PriorityP4},
		{"ADDON-001", PriorityP4},
		{"COREDNS-001", PriorityP4},
		{"EKS-NG-001", PriorityP4},
		{"EKS-NG-003", PriorityP4},
		{"EKS-NG-004", PriorityP4},
		{"EKS-INSIGHT-002", PriorityP4},
		{"EKS-INSIGHT-003", PriorityP4},

		{"SOME-UNREGISTERED-RULE", PriorityP4}, // defensive default, never expected in practice
	}
	for _, tc := range cases {
		got := AssignPriority(Finding{RuleID: tc.ruleID})
		if Priority(got.Priority) != tc.want {
			t.Errorf("AssignPriority(%s).Priority = %s, want %s", tc.ruleID, got.Priority, tc.want)
		}
		if got.PriorityReason != priorityReasons[tc.want] {
			t.Errorf("AssignPriority(%s).PriorityReason = %q, want %q", tc.ruleID, got.PriorityReason, priorityReasons[tc.want])
		}
	}
}

func TestAssignPriority_GlobalBlockerOverridesToP1RegardlessOfRuleID(t *testing.T) {
	for _, ruleID := range []string{"WH-002", "PDB-001", "ADDON-001", "SOME-UNREGISTERED-RULE"} {
		got := AssignPriority(Finding{RuleID: ruleID, GlobalBlocker: true})
		if Priority(got.Priority) != PriorityP1 {
			t.Errorf("AssignPriority(%s, GlobalBlocker=true).Priority = %s, want P1", ruleID, got.Priority)
		}
		if got.AffectedScope != "global" {
			t.Errorf("AssignPriority(%s, GlobalBlocker=true).AffectedScope = %q, want %q", ruleID, got.AffectedScope, "global")
		}
		if got.CanUpgradeContinue {
			t.Errorf("AssignPriority(%s, GlobalBlocker=true).CanUpgradeContinue = true, want false", ruleID)
		}
	}
}

func TestAssignPriority_CanUpgradeContinueFalseForBlockers(t *testing.T) {
	cases := []struct {
		ruleID        string
		severity      Severity
		globalBlocker bool
		wantContinue  bool
	}{
		{"WH-002", SeverityWarning, true, false},   // P1 global blocker
		{"API-001", SeverityBlocker, false, false}, // P2 blocker
		{"PDB-001", SeverityBlocker, false, false}, // P3 blocker
		{"PDB-001", SeverityWarning, false, true},  // P3 warning
		{"ADDON-001", SeverityWarning, false, true},
	}
	for _, tc := range cases {
		got := AssignPriority(Finding{RuleID: tc.ruleID, Severity: tc.severity, GlobalBlocker: tc.globalBlocker})
		if got.CanUpgradeContinue != tc.wantContinue {
			t.Errorf("AssignPriority(%s, Severity=%s, GlobalBlocker=%v).CanUpgradeContinue = %v, want %v", tc.ruleID, tc.severity, tc.globalBlocker, got.CanUpgradeContinue, tc.wantContinue)
		}
	}
}

func TestAssignPriority_AffectedScopeSetPerRule(t *testing.T) {
	cases := map[string]string{
		"NET-002":   "global",
		"PDB-001":   "workload",
		"NODE-001":  "node",
		"ADDON-001": "addon",
	}
	for ruleID, want := range cases {
		got := AssignPriority(Finding{RuleID: ruleID})
		if got.AffectedScope != want {
			t.Errorf("AssignPriority(%s).AffectedScope = %q, want %q", ruleID, got.AffectedScope, want)
		}
	}
}

func TestPriorityRank_OrderP1ThroughP4ThenUnknown(t *testing.T) {
	ranked := []int{
		PriorityRank(string(PriorityP1)),
		PriorityRank(string(PriorityP2)),
		PriorityRank(string(PriorityP3)),
		PriorityRank(string(PriorityP4)),
	}
	for i := 1; i < len(ranked); i++ {
		if ranked[i-1] >= ranked[i] {
			t.Errorf("PriorityRank not strictly increasing at index %d: %v", i, ranked)
		}
	}
	// Unrecognized values (including a Finding never run through
	// AssignPriority) all sort after P4, equally with each other — there's
	// no meaningful order among "unknown," just "known priorities first."
	worst := ranked[len(ranked)-1]
	for _, unknown := range []string{"", "not-a-real-priority"} {
		if r := PriorityRank(unknown); r <= worst {
			t.Errorf("PriorityRank(%q) = %d, want > P4's rank %d", unknown, r, worst)
		}
	}
}

// TestNewReport_AssignsPriorityToEveryFinding guards the central hook: any
// caller that builds a Report via NewReport gets every finding's Priority
// populated for free, without needing to remember to call AssignPriority
// itself.
func TestNewReport_AssignsPriorityToEveryFinding(t *testing.T) {
	fs := []Finding{
		{RuleID: "WH-002", Severity: SeverityBlocker, Confidence: TierStaticCertain, Message: "m", Resources: []ResourceReference{LiveResource("Test", ScopeCluster, "", "a", "uid-a")}, Fingerprint: "fp-a", GlobalBlocker: true},
		{RuleID: "ADDON-001", Severity: SeverityBlocker, Confidence: TierProviderReported, Message: "m", Resources: []ResourceReference{LiveResource("Test", ScopeCluster, "", "b", "uid-b")}, Fingerprint: "fp-b"},
	}
	r := NewReport("1.34", "cluster", "eks", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), fs)

	if r.Findings[0].Priority != string(PriorityP1) {
		t.Errorf("Findings[0].Priority = %q, want P1 (GlobalBlocker)", r.Findings[0].Priority)
	}
	if r.Findings[1].Priority != string(PriorityP4) {
		t.Errorf("Findings[1].Priority = %q, want P4 (ADDON-001)", r.Findings[1].Priority)
	}
	for i, f := range r.Findings {
		if f.PriorityReason == "" {
			t.Errorf("Findings[%d].PriorityReason is empty, want it set by NewReport", i)
		}
	}
}
