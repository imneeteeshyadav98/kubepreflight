// package findings_test (external, not findings) so this file can import
// internal/rules without an import cycle — internal/rules already imports
// internal/findings.
package findings_test

import (
	"testing"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
	"github.com/imneeteeshyadav98/kubepreflight/internal/rules"
)

// TestEveryRegisteredRuleHasAnExplicitPriority guards
// priorityByRuleID (internal/findings/priority.go) against silently
// drifting from the live rule registry: a newly added rule that's
// forgotten from the priority table would default to P4 in AssignPriority
// without this test catching it — this makes that an explicit failure
// instead of a silent, possibly-wrong default.
func TestEveryRegisteredRuleHasAnExplicitPriority(t *testing.T) {
	registry := rules.NewDefaultRegistry()
	for _, ruleID := range registry.RuleIDs() {
		if !findings.HasExplicitPriorityMapping(ruleID) {
			t.Errorf("rule %s has no explicit entry in priorityByRuleID (internal/findings/priority.go) — it's silently landing on the P4 fallback; add a deliberate mapping", ruleID)
		}
	}
}
