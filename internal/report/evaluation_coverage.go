package report

import (
	"fmt"
	"sort"
	"strings"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

// EvaluationCoverage is the shared, renderer-agnostic summary of how
// completely this report's rule universe was actually evaluated --
// computed once here and consumed identically by the terminal, Markdown,
// and HTML renderers, so the three surfaces can never drift into
// disagreeing about what "evaluated" means (the whole reason this type
// exists rather than three copies of this aggregation logic). It composes
// with, and never duplicates or contradicts, findings.UpgradeReadinessSummary
// -- that type already answers "did any evaluated rule in this category
// produce a blocker/warning"; this type answers the orthogonal question
// "was the rule even evaluated in the first place." See
// findings.RuleExecutionRecord for the underlying per-rule facts this
// aggregates, and docs/roadmap/v1.3.0-scope-audit.md for why
// RuleApplicability (design-time: was this rule even in scope for this scan
// mode) and RuleExecutionState (runtime: did it actually run, and how) are
// kept as two separate axes rather than collapsed into one enum.
type EvaluationCoverage struct {
	// HasData is false only when the report carries zero RuleExecutions
	// records -- e.g. a pre-v1.3.0 document that was never passed through
	// comparison.LoadAndNormalize, or a malformed/raw input that bypassed
	// normalization entirely. Every renderer must check this before
	// rendering anything else derived from this type: with no data there is
	// nothing honest to summarize, and the only safe behavior is to render
	// nothing at all -- never a fabricated "fully evaluated" claim built
	// from an empty set. See BuildEvaluationCoverage.
	HasData bool

	// CoverageLabel is "Complete" only when every applicable rule evaluated
	// cleanly -- no applicable rule anywhere is not_evaluated,
	// insufficient_evidence, or failed. "Partial" otherwise. This is
	// deliberately never derived from finding counts: a report can have
	// zero findings and still be Partial coverage (that mismatch is exactly
	// the bug this PR exists to make visible -- see ruleOutcomeLabel).
	CoverageLabel string

	TotalRules           int
	Evaluated            int
	NotEvaluated         int
	InsufficientEvidence int
	Failed               int
	NotApplicable        int

	// Source is the exact display string: "Native" for a report this build
	// computed RuleExecutions for directly, or "Normalized from legacy
	// report" when Report.RuleExecutionsNormalized backfilled it from a
	// pre-v1.3.0 document with no native execution bookkeeping at all -- see
	// internal/comparison/normalize.go's normalizeRuleExecutions. Readers
	// must be able to tell these apart at a glance: normalized data is an
	// inference from finding presence/absence, never this scan's own
	// evidence.
	Source string
	// Normalized mirrors Report.RuleExecutionsNormalized, exposed separately
	// from Source (a plain display string) so a renderer can key a distinct
	// visual treatment (an HTML badge color, a Markdown notice block) off a
	// bool without string-comparing Source's exact text.
	Normalized bool
}

// BuildEvaluationCoverage aggregates r.RuleExecutions into the shared
// summary every renderer displays. Applicability is checked before State:
// RunAllWithExecutions (internal/rules/rule.go) always pairs
// Applicability: not_applicable with State: not_evaluated for a rule that
// was never registered for this scan mode -- a design-time exclusion, not a
// runtime coverage gap -- so those rules are counted under NotApplicable,
// never double-counted under NotEvaluated too. A legacy-normalized report
// can, unlike a native one, have Applicability: applicable paired with
// State: not_evaluated (see normalizeRuleExecutions) -- that combination
// correctly lands in NotEvaluated here, exactly as it should: it is a real
// coverage gap, just one whose applicability happens to be unknown rather
// than affirmatively excluded.
func BuildEvaluationCoverage(r *findings.Report) EvaluationCoverage {
	if len(r.RuleExecutions) == 0 {
		return EvaluationCoverage{}
	}

	c := EvaluationCoverage{
		HasData:    true,
		TotalRules: len(r.RuleExecutions),
		Normalized: r.RuleExecutionsNormalized,
	}
	for _, rec := range r.RuleExecutions {
		if rec.Applicability == findings.ApplicabilityNotApplicable {
			c.NotApplicable++
			continue
		}
		switch rec.State {
		case findings.ExecutionEvaluated:
			c.Evaluated++
		case findings.ExecutionInsufficientEvidence:
			c.InsufficientEvidence++
		case findings.ExecutionFailed:
			c.Failed++
		default:
			// findings.ExecutionNotEvaluated, and conservatively any
			// future/unknown state value this build doesn't recognize yet --
			// counted as a coverage gap rather than silently dropped from
			// the total or mistaken for a clean pass.
			c.NotEvaluated++
		}
	}

	if c.NotEvaluated == 0 && c.InsufficientEvidence == 0 && c.Failed == 0 {
		c.CoverageLabel = "Complete"
	} else {
		c.CoverageLabel = "Partial"
	}
	if c.Normalized {
		c.Source = "Normalized from legacy report"
	} else {
		c.Source = "Native"
	}
	return c
}

// RuleExecutionRow is one rule's rendered execution record -- renderer-
// agnostic display strings only (no per-format markup beyond a small
// state-class tag HTML maps onto its own badge CSS), shared verbatim by
// terminal/Markdown/HTML so a rule's applicability/state/outcome/reason can
// never be worded differently across the three surfaces.
type RuleExecutionRow struct {
	RuleID string
	// Applicability is the exact display label: "Applicable" or "Not
	// applicable" -- see applicabilityLabel.
	Applicability string
	// State is the exact display label required by this PR: "Evaluated",
	// "Not evaluated", "Insufficient evidence", or "Failed" -- see
	// executionStateLabel. Deliberately never "Unknown" as an umbrella term,
	// even for a future/unrecognized state value.
	State string
	// StateClass is a small, renderer-agnostic status tag ("clean",
	// "not-evaluated", "insufficient", "blocked", "not-applicable") that
	// html.go maps onto its existing badge-* CSS classes. Terminal/Markdown
	// ignore this field entirely since they have no color rendering.
	StateClass string
	// Outcome is the field carrying this PR's core safety invariant: it
	// folds in whether the rule actually produced any findings, so a reader
	// can never mistake "zero findings" for "clean" unless the rule was
	// genuinely Evaluated. See ruleOutcomeLabel for the exact mapping.
	Outcome string
	// Reason is the raw, unescaped reason text from
	// findings.RuleExecutionRecord.Reason -- each renderer is responsible
	// for its own output-format-safe escaping (html/template does this
	// automatically for HTML; Markdown uses markdownEscape).
	Reason string
}

// BuildRuleExecutionRows renders every findings.RuleExecutionRecord in r
// into the shared per-rule display shape, sorted by RuleID for stable,
// diffable output regardless of the order RuleExecutions happened to be
// built in. Returns nil (never a partial/misleading slice) when r has no
// rule-execution data at all.
func BuildRuleExecutionRows(r *findings.Report) []RuleExecutionRow {
	if len(r.RuleExecutions) == 0 {
		return nil
	}

	findingCounts := make(map[string]int, len(r.Findings))
	for _, f := range r.Findings {
		findingCounts[f.RuleID]++
	}

	rows := make([]RuleExecutionRow, len(r.RuleExecutions))
	for i, rec := range r.RuleExecutions {
		rows[i] = RuleExecutionRow{
			RuleID:        rec.RuleID,
			Applicability: applicabilityLabel(rec.Applicability),
			State:         executionStateLabel(rec.State),
			StateClass:    ruleStateClass(rec.Applicability, rec.State),
			Outcome:       ruleOutcomeLabel(rec.Applicability, rec.State, findingCounts[rec.RuleID]),
			Reason:        rec.Reason,
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].RuleID < rows[j].RuleID })
	return rows
}

// applicabilityLabel and executionStateLabel are the exact, fixed display
// strings this PR requires for findings.RuleApplicability/
// RuleExecutionState -- deliberately avoiding "Unknown" as an umbrella term:
// every legitimate value gets its own explicit, distinct word, and even an
// unrecognized future value falls back to the most conservative label
// available rather than a vague catch-all.
func applicabilityLabel(a findings.RuleApplicability) string {
	switch a {
	case findings.ApplicabilityNotApplicable:
		return "Not applicable"
	default:
		// findings.ApplicabilityApplicable, and conservatively any future
		// value this build doesn't recognize yet.
		return "Applicable"
	}
}

func executionStateLabel(s findings.RuleExecutionState) string {
	switch s {
	case findings.ExecutionEvaluated:
		return "Evaluated"
	case findings.ExecutionInsufficientEvidence:
		return "Insufficient evidence"
	case findings.ExecutionFailed:
		return "Failed"
	default:
		// findings.ExecutionNotEvaluated, and conservatively any future
		// value this build doesn't recognize yet -- never silently rendered
		// as if it were a pass; "Not evaluated" is the most conservative
		// label available.
		return "Not evaluated"
	}
}

// ruleStateClass gives html.go a small, stable status tag to map onto its
// existing badge-* CSS classes -- kept here, next to the label logic it
// mirrors, so the two can never drift into disagreeing about which rows
// count as which state.
func ruleStateClass(applicability findings.RuleApplicability, state findings.RuleExecutionState) string {
	if applicability == findings.ApplicabilityNotApplicable {
		return "not-applicable"
	}
	switch state {
	case findings.ExecutionEvaluated:
		return "clean"
	case findings.ExecutionInsufficientEvidence:
		return "insufficient"
	case findings.ExecutionFailed:
		return "blocked"
	default:
		return "not-evaluated"
	}
}

// ruleOutcomeLabel is the presentation layer for this PR's core safety
// invariant:
//
//	0 findings + evaluated              -> "No issue detected"
//	0 findings + not_evaluated          -> "Not evaluated"
//	0 findings + insufficient_evidence  -> "Unable to verify"
//	0 findings + failed                 -> "Evaluation failed"
//
// A "clean" reading (zero findings) must only ever be shown when the rule
// was genuinely State: evaluated -- every other state renders its own
// distinct wording instead, so an absence of findings from a rule that
// wasn't evaluated, ran into insufficient evidence, or failed can never be
// misread as a pass. Applicability is checked first because
// RunAllWithExecutions always pairs not_applicable with State:
// not_evaluated (a design-time exclusion, not a runtime gap -- see
// internal/rules/rule.go) -- that case gets its own distinct wording rather
// than reading as a coverage gap that needs attention.
func ruleOutcomeLabel(applicability findings.RuleApplicability, state findings.RuleExecutionState, findingCount int) string {
	if applicability == findings.ApplicabilityNotApplicable {
		return "Not applicable"
	}
	switch state {
	case findings.ExecutionEvaluated:
		if findingCount == 0 {
			return "No issue detected"
		}
		return fmt.Sprintf("%d finding(s) reported", findingCount)
	case findings.ExecutionInsufficientEvidence:
		return "Unable to verify"
	case findings.ExecutionFailed:
		return "Evaluation failed"
	default:
		return "Not evaluated"
	}
}

// markdownEscape sanitizes free-form reason text for safe inclusion inside
// a Markdown table cell: an unescaped "|" would prematurely close the cell
// and corrupt every column after it, and unescaped *_`<> could trigger
// unintended emphasis, code spans, or -- rendered through a Markdown-to-HTML
// pipeline that permits raw inline HTML (e.g. GitHub-flavored Markdown) --
// tag injection. Reason text originates from rule error messages and
// static strings (see sanitizeRuleError, legacyEvaluatedReason), never from
// authored Markdown, so every one of these is escaped rather than
// interpreted. Uses a single-pass strings.Replacer, so replacement output
// (e.g. the backslash introduced by "|" -> "\|") is never re-scanned and
// double-escaped.
func markdownEscape(s string) string {
	if s == "" {
		return s
	}
	return markdownEscaper.Replace(s)
}

var markdownEscaper = strings.NewReplacer(
	"\\", "\\\\",
	"|", "\\|",
	"`", "\\`",
	"*", "\\*",
	"_", "\\_",
	"<", "&lt;",
	">", "&gt;",
	"&", "&amp;",
	"\n", " ",
	"\r", " ",
)
