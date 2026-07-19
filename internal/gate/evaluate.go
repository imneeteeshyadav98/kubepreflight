package gate

import (
	"github.com/imneeteeshyadav98/kubepreflight/internal/comparison"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

// verdictRank orders the three "confident" verdicts findings.Report.Result
// can return once evidence is known complete -- INCOMPLETE is deliberately
// absent here, since Evaluate's evidence-quality check always resolves to
// DecisionNeutral before this ranking is ever consulted for an incomplete
// report, exactly like the "don't guess" principle applied everywhere else
// findings.Report drives a decision.
var verdictRank = map[string]int{
	"CLEAN":                0,
	"PASSED_WITH_WARNINGS": 1,
	"BLOCKED":              2,
}

// Evaluate turns a baseline scan, a current scan, and their pre-computed
// comparison into a deterministic gate Result under policy. baseline and
// current must be the same two reports cmp was built from (Evaluate reads
// their Summary/coverage directly for facts comparison.Comparison doesn't
// carry on its own, such as the current scan's total warning count) --
// Evaluate never re-diffs findings itself.
func Evaluate(baseline, current *findings.Report, cmp *comparison.Comparison, policy Policy) Result {
	result := Result{
		SchemaVersion:    SchemaVersion,
		NewBlockers:      cmp.Summary.NewBlockers,
		NewWarnings:      countSeverity(cmp.New, findings.SeverityWarning),
		CurrentWarnings:  current.Summary.Warnings,
		ResolvedFindings: cmp.Summary.Resolved,
		ScoreDelta:       cmp.Summary.ReadinessScoreDelta,
	}

	// Evidence quality always wins over policy: a comparison built from an
	// incomplete scan (either side) or a target-version mismatch (which
	// comparison.Compare already flags via cmp.Warnings, since it makes
	// genuinely-unchanged findings look like a new+resolved pair) can't
	// honestly support a confident pass or fail -- matching the same
	// "don't guess" principle findings.Report.UpgradeApplicable and the
	// rollback recommendation engine already apply.
	if !baseline.IsComplete() || !current.IsComplete() || len(cmp.Warnings) > 0 {
		result.Decision = DecisionNeutral
		result.Reasons = []ReasonCode{ReasonInsufficientEvidence}
		return result
	}

	var reasons []ReasonCode
	if policy.FailOnNewBlockers && result.NewBlockers > 0 {
		reasons = append(reasons, ReasonNewBlockersDetected)
	}
	switch policy.WarningPolicy {
	case WarningPolicyFailOnNew:
		if result.NewWarnings > 0 {
			reasons = append(reasons, ReasonNewWarningsDetected)
		}
	case WarningPolicyFailOnAny:
		if result.CurrentWarnings > 0 {
			reasons = append(reasons, ReasonWarningsPresent)
		}
	}
	if policy.FailOnVerdictRegression && verdictRegressed(cmp.Summary.BaselineVerdict, cmp.Summary.CurrentVerdict) {
		reasons = append(reasons, ReasonReadinessVerdictRegressed)
	}
	if result.ScoreDelta < policy.MinimumScoreDelta {
		reasons = append(reasons, ReasonReadinessScoreRegressed)
	}

	if len(reasons) > 0 {
		result.Decision = DecisionFail
		result.Reasons = reasons
		return result
	}
	result.Decision = DecisionPass
	return result
}

func countSeverity(entries []comparison.Entry, severity findings.Severity) int {
	count := 0
	for _, e := range entries {
		if e.Severity == severity {
			count++
		}
	}
	return count
}

func verdictRegressed(baseline, current string) bool {
	return verdictRank[current] > verdictRank[baseline]
}
