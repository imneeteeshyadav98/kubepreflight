// package findings_test (external, not findings) so this file can import
// internal/rules without an import cycle — internal/rules already imports
// internal/findings. Mirrors priority_registry_test.go's exact pattern.
package findings_test

import (
	"testing"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
	"github.com/imneeteeshyadav98/kubepreflight/internal/rules"
)

// TestEveryRegisteredRuleHasAnExplicitCategory guards categoryByRuleID
// (internal/findings/report.go) against silently drifting from the live
// rule registry: a newly added rule that's forgotten from the category
// table would be silently excluded from every category's counts and the
// UpgradeReadinessSummary's score — this makes that an explicit failure
// instead of a silent gap.
func TestEveryRegisteredRuleHasAnExplicitCategory(t *testing.T) {
	registry := rules.NewDefaultRegistry()
	for _, ruleID := range registry.RuleIDs() {
		if !findings.HasExplicitCategoryMapping(ruleID) {
			t.Errorf("rule %s has no explicit entry in categoryByRuleID (internal/findings/report.go) — it's silently excluded from the Upgrade Readiness scorecard; add a deliberate category mapping", ruleID)
		}
	}
}
