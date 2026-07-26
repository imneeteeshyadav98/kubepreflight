// Package comparison diffs two findings.json documents (a baseline and a
// current scan of the same cluster/target) into new/resolved/changed/
// unchanged findings plus verdict and readiness-score movement. It has no
// CLI dependency so the Console, the GitHub Action, and any future API can
// reuse the same engine internal/cli/compare.go wires up.
package comparison

import "github.com/imneeteeshyadav98/kubepreflight/internal/findings"

// SchemaVersion identifies the comparison document format, matching the
// URN-style versioning internal/plan.ActionPlanSchemaVersion already
// established for kubepreflight's other standalone JSON documents (not
// Kubernetes's own "apiVersion" convention, which this project doesn't use
// anywhere else).
const SchemaVersion = "kubepreflight.io/scan-comparison/v1"

// Comparison is the top-level output document.
type Comparison struct {
	SchemaVersion string    `json:"schemaVersion"`
	Warnings      []string  `json:"warnings,omitempty"`
	Summary       Summary   `json:"summary"`
	New           []Entry   `json:"new"`
	Resolved      []Entry   `json:"resolved"`
	Changed       []Changed `json:"changed"`
	Unchanged     []Entry   `json:"unchanged"`
	// NotReEvaluated holds every baseline-only finding (present in baseline,
	// absent from current) whose responsible rule cannot be proven to have
	// evaluated cleanly in the current report -- see Compare's classification
	// logic and docs/roadmap/v1.3.0-scope-audit.md's PR 5/Decision 1. This is
	// an ADDITIVE fifth bucket alongside the original frozen four
	// (New/Resolved/Changed/Unchanged) -- comparison schema
	// "kubepreflight.io/scan-comparison/v1" is a frozen v1 surface per
	// docs/v1-compatibility-contract.md, and per Decision 2 in the scope
	// audit, an additive field under the existing schema version is the
	// approved, deprecation-policy-governed way to add it (no version-string
	// bump required for an additive change).
	//
	// The JSON key is deliberately the locked snake_case "not_re_evaluated",
	// not the camelCase "notReEvaluated" every other multi-word field on this
	// type uses -- Decision 1 in the scope audit fixes this exact wire
	// spelling for the comparison bucket, matching the precedent
	// findings.CoverageStatus already set (Go constant CoveragePartial, wire
	// value "partial"): the Go identifier and its JSON wire value are
	// independent naming surfaces here by deliberate, reviewed choice, not an
	// oversight.
	NotReEvaluated []Entry `json:"not_re_evaluated"`
}

// Summary is the at-a-glance verdict/score movement and counts.
type Summary struct {
	BaselineVerdict        string `json:"baselineVerdict"`
	CurrentVerdict         string `json:"currentVerdict"`
	BaselineUpgradeContext string `json:"baselineUpgradeContext,omitempty"`
	CurrentUpgradeContext  string `json:"currentUpgradeContext,omitempty"`
	VerdictChanged         bool   `json:"verdictChanged"`
	BaselineReadinessScore int    `json:"baselineReadinessScore"`
	CurrentReadinessScore  int    `json:"currentReadinessScore"`
	ReadinessScoreDelta    int    `json:"readinessScoreDelta"`
	New                    int    `json:"new"`
	Resolved               int    `json:"resolved"`
	Changed                int    `json:"changed"`
	Unchanged              int    `json:"unchanged"`
	NewBlockers            int    `json:"newBlockers"`
	ResolvedBlockers       int    `json:"resolvedBlockers"`
	// NotReEvaluated is len(Comparison.NotReEvaluated) -- see that field's
	// doc comment for why this count is never folded into Resolved, and why
	// its JSON key is the locked snake_case "not_re_evaluated" rather than
	// this type's usual camelCase.
	NotReEvaluated int `json:"not_re_evaluated"`
}

// NotReEvaluatedLabel and NotReEvaluatedExplanation are the exact, fixed
// display strings every rendering surface (terminal, Markdown, Console) must
// use verbatim for the not_re_evaluated bucket -- see
// docs/roadmap/v1.3.0-scope-audit.md, PR 5's acceptance criteria. Kept here,
// next to the field they describe, so every renderer sources the same
// string rather than each spelling it out independently.
const (
	NotReEvaluatedLabel       = "Not re-evaluated"
	NotReEvaluatedExplanation = "The finding was present in the baseline, but its rule was not successfully evaluated in the current report, so resolution cannot be confirmed."
)

// Entry wraps one finding in the New/Resolved/Unchanged buckets. It's the
// full finding, not a summary — a comparison consumer (Console, change
// ticket) needs the same evidence/remediation a plain scan would show.
type Entry struct {
	findings.Finding
}

// Changed is one finding present in both scans (same fingerprint) with at
// least one tracked field different between them. Tracked fields are:
// severity, priority, confidence, canUpgradeContinue, affectedScope,
// ruleId, and resource identity (the last two are defensive -- Fingerprint
// itself already hashes ruleID and each resource's concept key, so two
// findings genuinely differing in either could never share a fingerprint
// in practice, but the check costs nothing and stays correct if that ever
// changes). Evidence/remediation text is deliberately NOT tracked -- see
// compare.go's diffFinding -- so a copy-edit to a remediation string never
// shows up as a "Changed" finding, only real decision-relevant movement.
type Changed struct {
	Fingerprint string                       `json:"fingerprint"`
	RuleID      string                       `json:"ruleId"`
	Resources   []findings.ResourceReference `json:"resources"`
	Changes     map[string]FieldChange       `json:"changes"`
}

// FieldChange is one tracked field's before/after value, always rendered
// as strings so severity/priority/bool/scope changes share one shape.
type FieldChange struct {
	Before string `json:"before"`
	After  string `json:"after"`
}
