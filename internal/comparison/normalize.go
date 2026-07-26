package comparison

import (
	"encoding/json"
	"fmt"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
	"github.com/imneeteeshyadav98/kubepreflight/internal/v1compat"
)

// LoadAndNormalize parses raw findings.json bytes and backfills fields an
// older schema version might not have written, so a baseline captured
// months ago can still be compared against a current scan:
//
//   - missing Priority/PriorityReason/AffectedScope/CanUpgradeContinue on a
//     finding are derived via findings.AssignPriority, the same function
//     NewReport itself uses -- never guessed, never left zero-valued.
//   - a missing UpgradeReadiness/APICompatibility summary is rebuilt from
//     the document's own findings and verdict, exactly as NewReport builds
//     it, so an old document without these fields still gets an accurate
//     one instead of a nil comparison target.
//   - ScanCoverage is preserved exactly as recorded -- an old document's
//     INCOMPLETE status is never silently treated as CLEAN just because a
//     newer field happened to be absent too.
//   - unknown/future fields are tolerated (encoding/json already ignores
//     them; this package never rejects a document just for having fields
//     it doesn't know about).
//
// A document that isn't recognizable as a findings.json at all --
// malformed JSON, or missing the fields every schema version has always
// had (schemaVersion, targetVersion) -- is an explicit error, never a
// silently-empty comparison.
func LoadAndNormalize(raw []byte) (*findings.Report, error) {
	var r findings.Report
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("parsing findings document: %w", err)
	}
	if r.SchemaVersion == "" || r.TargetVersion == "" {
		return nil, fmt.Errorf("not a recognizable findings.json document: missing schemaVersion/targetVersion")
	}

	for i, f := range r.Findings {
		if f.Priority == "" {
			r.Findings[i] = findings.AssignPriority(f)
		}
	}

	verdict := r.Result()
	if r.APICompatibility == nil {
		r.APICompatibility = findings.BuildAPICompatibilitySummary(r.Findings)
	}
	if r.UpgradeReadiness == nil {
		r.UpgradeReadiness = findings.BuildUpgradeReadinessSummary(r.Findings, verdict)
	}
	normalizeRuleExecutions(&r)

	return &r, nil
}

// legacyEvaluatedReason and legacyNotEvaluatedReason are the Reason strings
// normalizeRuleExecutions attaches to every backfilled RuleExecutionRecord.
// Both are deliberately explicit that the state is an inference from finding
// presence/absence, never a native execution record -- see
// normalizeRuleExecutions' own doc comment for why this distinction is the
// entire point of this PR.
const (
	legacyEvaluatedReason    = "legacy report contains a finding from this rule; execution metadata was backfilled from finding presence, not a native execution record"
	legacyNotEvaluatedReason = "legacy report does not contain native rule-execution metadata; absence of a finding was not treated as evaluated-and-clean"
)

// normalizeRuleExecutions backfills Report.RuleExecutions for a document that
// predates PR 1 (docs/roadmap/v1.3.0-scope-audit.md's approved 8-PR
// sequence) and therefore never natively computed it.
//
// Detection is presence-based, not SchemaVersion-based, matching the exact
// convention every other backfill in this function already uses (Priority:
// `f.Priority == ""`; APICompatibility/UpgradeReadiness: `== nil`) --
// SchemaVersion alone cannot distinguish a pre-PR-1 document from a
// PR-1-native one, since PR 1 deliberately left findings.SchemaVersion at
// "1.0" (that bump is PR 7's job, per the locked scope doc's Decision 2).
// `len(r.RuleExecutions) == 0` is therefore the only signal available today,
// and it deliberately treats an empty-but-present `"ruleExecutions": []` the
// same as a wholly absent field: there is no further signal (no build/tool
// version stamp anywhere in Report) that could distinguish "a PR-1-native
// producer emitted a genuinely empty array" (which shouldn't happen in
// practice -- the rule universe always has entries) from "this document
// predates the field" -- so both are conservatively treated as needing
// backfill rather than silently left as a bare, unmarked empty array. This
// also makes the function idempotent for free: calling it again on an
// already-backfilled report (RuleExecutions already holds one entry per
// universe rule ID) takes the same early-return path and changes nothing.
//
// THE CORE SAFETY RULE this function exists to enforce: a legacy report's
// missing rule-execution metadata must never be read as "every rule
// evaluated cleanly." A rule ID is only ever marked evaluated when the
// legacy document actually contains at least one finding from it --
// evidence the rule genuinely ran and produced something. Every other rule
// ID -- including every rule that ran cleanly with zero findings, which is
// indistinguishable from "never ran" in a document with no native execution
// bookkeeping -- is marked not_evaluated, never evaluated. Absence of a
// finding is NOT evidence of a clean pass; it is only evidence of absence.
//
// One canonical record is built for every ID in v1compat.ExpectedRuleIDs(),
// the full current rule universe, not just the rule IDs the legacy
// document's own findings happen to mention -- an old report can predate
// rules that didn't exist yet, and every one of those must still get an
// explicit not_evaluated record rather than being silently missing.
//
// v1compat.ExpectedRuleIDs() is used here in place of the more obvious
// rules.AllRuleIDs() for a deliberate, documented reason: internal/rules'
// own test files (package rules, e.g. addon001_test.go) already import
// internal/comparison (to exercise Compare() against ADDON-001/ADDON-002
// scenarios), so internal/comparison importing internal/rules back would be
// a genuine compile-time import cycle for `go test ./internal/rules`
// (confirmed by attempting exactly that import: "import cycle not allowed
// in test"). internal/v1compat has zero internal package dependencies (only
// stdlib), so importing it here is cycle-free. Its ExpectedRuleIDs() list is
// not a second, independently-drifting source of truth: internal/cli's
// TestV1CompatibilityContractCurrentImplementation
// (v1contract_test.go) feeds rules.NewDefaultRegistry().RuleIDs() (the same
// slice rules.AllRuleIDs() returns) through v1compat.Check, which asserts
// exact, order-sensitive equality against v1compat.ExpectedRuleIDs()
// (contract.go's compareStringList joins both slices and requires a byte
// match) -- so any drift between the two lists already fails CI
// independently of this package.
//
// Conversely, a legacy finding whose RuleID is NOT in the current rule
// universe (e.g. a rule renamed or removed at some point -- no such
// rename/removal has actually happened in this project's history, confirmed
// by git log, but the code must not assume that stays true forever)
// contributes nothing to this bookkeeping: the finding itself is left
// completely untouched in r.Findings, only the derived, fixed-length
// RuleExecutions array is unaffected by it, matching how categoryByRuleID
// (internal/findings/report.go) already silently ignores an unrecognized
// rule ID (`name, ok := categoryByRuleID[f.RuleID]; if !ok { continue }`)
// rather than inventing a new category slot for it.
//
// Duplicate findings from the same rule ID (e.g. three separate WH-005
// findings) collapse into exactly one RuleExecutionRecord for that rule ID,
// never three -- the loop below tracks presence via a map keyed on rule ID,
// not a per-finding append.
func normalizeRuleExecutions(r *findings.Report) {
	if len(r.RuleExecutions) > 0 {
		return
	}

	hasFinding := make(map[string]bool, len(r.Findings))
	for _, f := range r.Findings {
		hasFinding[f.RuleID] = true
	}

	universe := v1compat.ExpectedRuleIDs()
	records := make([]findings.RuleExecutionRecord, 0, len(universe))
	for _, ruleID := range universe {
		if hasFinding[ruleID] {
			records = append(records, findings.RuleExecutionRecord{
				RuleID:        ruleID,
				Applicability: findings.ApplicabilityApplicable,
				State:         findings.ExecutionEvaluated,
				Reason:        legacyEvaluatedReason,
			})
			continue
		}
		records = append(records, findings.RuleExecutionRecord{
			RuleID:        ruleID,
			Applicability: findings.ApplicabilityApplicable,
			State:         findings.ExecutionNotEvaluated,
			Reason:        legacyNotEvaluatedReason,
		})
	}

	r.RuleExecutions = records
	r.RuleExecutionsNormalized = true
}
