// Package gate turns a comparison.Comparison into a deterministic CI
// pass/fail/neutral decision under a configurable policy. It is the "CI
// decision" layer: comparison.Comparison stays the "facts" (what changed
// between two scans), and gate.Result is the separate judgment a caller
// (GitHub Action, any other CI integration) renders those facts into.
// This package never re-diffs findings, never touches comparison.Compare's
// fingerprint matching, and has no knowledge of GitHub, Actions, or any
// other delivery surface.
package gate

// SchemaVersion identifies the gate result document format, matching the
// URN-style versioning comparison.SchemaVersion/plan.ActionPlanSchemaVersion
// already established for this project's standalone JSON documents.
const SchemaVersion = "kubepreflight.io/comparison-gate/v1"

// Decision is the final CI outcome. It deliberately has three states, not
// two: a comparison built from incomplete evidence (partial coverage on
// either side, or a target-version mismatch that makes the diff itself
// unreliable) must never present as a confident "pass" or "fail" -- see
// Evaluate's evidence-quality check, which always wins over policy.
type Decision string

const (
	DecisionPass    Decision = "pass"
	DecisionFail    Decision = "fail"
	DecisionNeutral Decision = "neutral"
)

// WarningPolicy controls how Warning-severity findings affect the
// decision. Only Blockers are fatal by default (matching every other
// exit-code/verdict decision this project already makes) -- Warnings
// require an explicit opt-in to become gating.
type WarningPolicy string

const (
	// WarningPolicyIgnore never fails the gate on warnings alone. Default.
	WarningPolicyIgnore WarningPolicy = "ignore"
	// WarningPolicyFailOnNew fails when the current scan introduces at
	// least one warning the baseline didn't have, regardless of how many
	// pre-existing warnings remain.
	WarningPolicyFailOnNew WarningPolicy = "fail_on_new"
	// WarningPolicyFailOnAny fails when the current scan has any warning
	// at all, new or pre-existing -- the strictest setting.
	WarningPolicyFailOnAny WarningPolicy = "fail_on_any"
)

// ReasonCode is a deterministic, machine-checkable cause the decision can
// cite -- never free-form text, so a caller (or a test) can assert on the
// exact cause without parsing a sentence.
type ReasonCode string

const (
	ReasonNewBlockersDetected       ReasonCode = "NEW_BLOCKERS_DETECTED"
	ReasonNewWarningsDetected       ReasonCode = "NEW_WARNINGS_DETECTED"
	ReasonWarningsPresent           ReasonCode = "WARNINGS_PRESENT"
	ReasonReadinessVerdictRegressed ReasonCode = "READINESS_VERDICT_REGRESSED"
	ReasonReadinessScoreRegressed   ReasonCode = "READINESS_SCORE_REGRESSED"
	ReasonInsufficientEvidence      ReasonCode = "INSUFFICIENT_EVIDENCE"
)

// Policy is the configurable gate contract. Every field's zero value is
// the most permissive choice available for that field except
// FailOnNewBlockers, which defaults true even at the Go zero value's
// opposite (false) -- callers should use DefaultPolicy rather than a bare
// Policy{} literal to get the intended defaults.
type Policy struct {
	// FailOnNewBlockers fails the gate when the current scan introduces
	// at least one effective upgrade blocker the baseline didn't have.
	FailOnNewBlockers bool `json:"failOnNewBlockers"`
	// WarningPolicy controls whether/when warnings gate the decision.
	WarningPolicy WarningPolicy `json:"warningPolicy"`
	// FailOnVerdictRegression fails the gate when the overall verdict
	// gets strictly worse (CLEAN -> PASSED_WITH_WARNINGS -> BLOCKED),
	// independent of the specific blocker/warning counts driving it.
	FailOnVerdictRegression bool `json:"failOnVerdictRegression"`
	// MinimumScoreDelta is the lowest readiness-score movement (current
	// minus baseline) that still passes; anything below it fails. 0
	// means the score must not drop at all; a negative value tolerates
	// some regression before failing.
	MinimumScoreDelta int `json:"minimumScoreDelta"`
}

// DefaultPolicy is the conservative starting point: any new blocker or
// verdict regression fails the gate, warnings alone never do, and the
// readiness score must not drop.
func DefaultPolicy() Policy {
	return Policy{
		FailOnNewBlockers:       true,
		WarningPolicy:           WarningPolicyIgnore,
		FailOnVerdictRegression: true,
		MinimumScoreDelta:       0,
	}
}

// Result is the gate's output document -- the CI decision plus the raw
// counts a caller can render without recomputing anything.
//
// EvaluationCoverage and EvaluationAdvisories are additive, presentation-
// only fields: they carry context about how completely current's rule
// universe was evaluated this scan (plus the comparison-specific
// not_re_evaluated count), but neither one ever participates in Decision.
// Adding them changes no existing field, no Decision value, and no exit
// code for any input that produced a given Decision before these fields
// existed -- see Evaluate's coverage-invariance tests.
type Result struct {
	SchemaVersion string       `json:"schemaVersion"`
	Decision      Decision     `json:"decision"`
	Reasons       []ReasonCode `json:"reasons,omitempty"`

	NewBlockers      int `json:"newBlockers"`
	NewWarnings      int `json:"newWarnings"`
	CurrentWarnings  int `json:"currentWarnings"`
	ResolvedFindings int `json:"resolvedFindings"`
	ScoreDelta       int `json:"scoreDelta"`

	// EvaluationCoverage is always populated, on every Evaluate call and
	// every Decision path (including the early DecisionNeutral return for
	// insufficient evidence) -- see report.BuildEvaluationCoverage, the
	// single shared aggregation this reuses verbatim rather than
	// recomputing any count independently.
	EvaluationCoverage EvaluationCoverage `json:"evaluationCoverage"`
	// EvaluationAdvisories is the human-readable text a CLI/Console caller
	// shows alongside Decision -- 0, 1, or 2 entries: a coverage advisory
	// (report.EvaluationCoverage.Advisory, omitted entirely when coverage
	// is complete) and/or a not_re_evaluated advisory (omitted when that
	// count is zero). Nil, not an empty non-nil slice, when there is
	// nothing to caution about.
	EvaluationAdvisories []string `json:"evaluationAdvisories,omitempty"`
}

// EvaluationCoverage is Result's additive summary of how completely
// current's decision coverage was this scan, plus the comparison-specific
// NotReEvaluated count. JSON field names are deliberately spelled out in
// full (no abbreviation) to match this package's existing Result
// field-naming style.
type EvaluationCoverage struct {
	// Status is report.OverallCoverage.Status's exact wire value: "complete",
	// "partial", "unavailable", or "normalized_legacy" -- report.BuildOverallCoverage(
	// report.BuildEvaluationCoverage(current), current.Coverage), never
	// recomputed here. This is the COMBINED rule-execution +
	// evidence-plane status, not rule-execution coverage alone: a report
	// can have every rule read State: evaluated (report.CoverageStatusComplete
	// at the rule-execution level) while an evidence plane (e.g. AWS under
	// reduced IAM) came back partial, and Status here must still read
	// "partial" in that case -- see report.OverallCoverage's doc comment
	// for the exact bug this fixes and the precedence rules that compute
	// this value.
	Status string `json:"status"`
	// NotEvaluated/InsufficientEvidence/Failed/Normalized remain the plain
	// rule-execution-only counts from report.BuildEvaluationCoverage(current)
	// (never recomputed here) -- still accurate and useful at that level
	// even when Status above reads partial for evidence-plane reasons
	// alone; see DegradedPlanes for the evidence-plane half of the picture.
	NotEvaluated         int  `json:"notEvaluated"`
	InsufficientEvidence int  `json:"insufficientEvidence"`
	Failed               int  `json:"failed"`
	Normalized           bool `json:"normalized"`
	// DegradedPlanes lists, in Kubernetes/AWS/Manifests order, every
	// evidence plane that was actually attempted this scan but came back
	// partial -- report.OverallCoverage.DegradedPlanes verbatim. This is
	// what lets a caller see *why* Status reads "partial" even when every
	// count above is zero (the reduced-IAM certification scenario this fix
	// addresses: 0 NotEvaluated/InsufficientEvidence/Failed, but
	// DegradedPlanes: ["AWS"]). Omitted/empty whenever no plane is partial.
	DegradedPlanes []string `json:"degradedPlanes,omitempty"`
	// NotReEvaluated is comparison.Summary.NotReEvaluated verbatim:
	// baseline findings whose rule wasn't proven to have evaluated cleanly
	// in current, so comparison.Compare could not confirm resolution --
	// see comparison.NotReEvaluatedExplanation for the operator-facing
	// version of this fact.
	NotReEvaluated int `json:"notReEvaluated"`
}
