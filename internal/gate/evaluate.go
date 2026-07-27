package gate

import (
	"fmt"

	"github.com/imneeteeshyadav98/kubepreflight/internal/comparison"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
	"github.com/imneeteeshyadav98/kubepreflight/internal/report"
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
	// Evaluation coverage/advisories are additive presentation fields, set
	// unconditionally before either return path below (including the early
	// DecisionNeutral return) so they're populated identically regardless
	// of Decision -- see buildEvaluationPresentation and Result's own doc
	// comment for why nothing here may ever influence Decision.
	result.EvaluationCoverage, result.EvaluationAdvisories = buildEvaluationPresentation(current, cmp)

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

// buildEvaluationPresentation maps report.BuildEvaluationCoverage(current)
// -- the single shared rule-execution aggregation terminal/Markdown/HTML/
// Console also read from -- composed with report.BuildOverallCoverage
// (current's rule-execution coverage folded together with current.Coverage,
// the evidence-plane coverage) into this package's slim, JSON-tagged
// EvaluationCoverage, and builds the 0-2 human-readable advisory strings a
// caller shows alongside Decision. Status/DegradedPlanes/the advisory text
// come from the COMBINED report.OverallCoverage (see its doc comment for
// why: a report can read rule-execution coverage "complete" while an
// evidence plane came back partial, and this is the one call site every
// gate/CLI consumer reads that combined verdict from, so it can never
// independently recompute a different answer). NotEvaluated/
// InsufficientEvidence/Failed/Normalized stay the plain rule-execution-only
// counts. Nothing here is recomputed independently of either Build* call,
// and nothing here is fed back into Decision.
func buildEvaluationPresentation(current *findings.Report, cmp *comparison.Comparison) (EvaluationCoverage, []string) {
	ruleCov := report.BuildEvaluationCoverage(current)
	overallCov := report.BuildOverallCoverage(ruleCov, current.Coverage)
	out := EvaluationCoverage{
		Status:               string(overallCov.Status),
		NotEvaluated:         ruleCov.NotEvaluated,
		InsufficientEvidence: ruleCov.InsufficientEvidence,
		Failed:               ruleCov.Failed,
		Normalized:           ruleCov.Normalized,
		DegradedPlanes:       overallCov.DegradedPlanes,
		NotReEvaluated:       cmp.Summary.NotReEvaluated,
	}

	var advisories []string
	if advisory := overallCov.Advisory(); advisory != "" {
		advisories = append(advisories, advisory)
	}
	if cmp.Summary.NotReEvaluated > 0 {
		advisories = append(advisories, fmt.Sprintf(
			"%d baseline finding(s) were not re-evaluated this scan: %s",
			cmp.Summary.NotReEvaluated, comparison.NotReEvaluatedExplanation))
	}
	return out, advisories
}
