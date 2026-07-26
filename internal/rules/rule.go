// Package rules defines the check interface and registry every deterministic
// check (API-001, WH-001, etc.) registers against.
package rules

import (
	"fmt"
	"strings"

	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/aws"
	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/k8s"
	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/manifest"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

// ScanContext bundles every collector's evidence for a single scan. AWS and
// Manifests are nil whenever that plane wasn't attempted or was gracefully
// skipped — rules that depend on them must check for nil and simply
// produce no findings, never error. This mirrors the deep dive's "five
// evidence planes feed one rules engine" architecture (Section 13.1):
// collectors stay independent, rules merge whatever evidence happens to be
// available.
type ScanContext struct {
	K8s            *k8s.Snapshot
	AWS            *aws.Snapshot
	Manifests      *manifest.Snapshot
	UpgradeContext findings.UpgradeContext
}

// Rule is a single deterministic check evaluated against a ScanContext for
// a given upgrade target version.
type Rule interface {
	// ID returns the rule's stable identifier, e.g. "API-001".
	ID() string
	// Evaluate inspects the scan context and returns zero or more findings.
	Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error)
}

// Registry holds the set of rules a scan will run.
type Registry struct {
	rules []Rule
}

// NewRegistry returns an empty registry. Rules are added via Register.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a rule to the registry.
func (r *Registry) Register(rule Rule) {
	r.rules = append(r.rules, rule)
}

// RuleIDs returns the ID of every registered rule, in registration order.
// Used by internal/plan's tests to guard against a newly registered rule
// silently missing an explicit projection-policy decision.
func (r *Registry) RuleIDs() []string {
	ids := make([]string, len(r.rules))
	for i, rule := range r.rules {
		ids[i] = rule.ID()
	}
	return ids
}

// RunAll evaluates every registered rule against the scan context and
// returns the combined finding list. A rule error does not abort the
// others.
//
// KNOWN, INTENTIONALLY UNFIXED LIMITATION: only the very first rule (in
// registration order) to return an error has that error retained as
// firstErr -- every subsequent rule's error is silently discarded, its only
// trace being that its findings are dropped too. This is a confirmed,
// pre-existing gap (docs/roadmap/v1.3.0-scope-audit.md, "Confirmed product
// gaps" #9 and Decision 5) that v1.3.0 deliberately does NOT fix here: the
// fix (accumulate every rule's error, e.g. via errors.Join) is scoped as an
// independent, small v1.2.2 patch candidate, orthogonal to the
// RuleExecutionRecord model RunAllWithExecutions adds below. Do not "fix"
// this by changing what RunAll returns -- doing so here would be exactly
// the out-of-scope change the locked release document forbids for this PR.
func (r *Registry) RunAll(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	all, firstErr, _ := r.runAll(sc, targetVersion)
	return all, firstErr
}

// runAll is the shared core of RunAll and RunAllWithExecutions. It
// preserves RunAll's existing first-error-only behavior byte-for-byte (same
// loop, same conditions, same values) -- this refactor changes nothing
// about what RunAll itself returns. It additionally tracks which single
// rule ID the surviving firstErr came from (firstErrRuleID), which
// RunAllWithExecutions needs so it can attribute State: failed to the one
// rule RunAll's existing logic actually preserved an error for, rather than
// guessing or over-attributing it to every rule that happened to error.
func (r *Registry) runAll(sc *ScanContext, targetVersion string) (all []findings.Finding, firstErr error, firstErrRuleID string) {
	for _, rule := range r.rules {
		fs, err := rule.Evaluate(sc, targetVersion)
		if err != nil && firstErr == nil {
			firstErr = err
			firstErrRuleID = rule.ID()
		}
		for i, f := range fs {
			if err := f.Validate(); err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("%s finding %d: %w", rule.ID(), i, err)
					firstErrRuleID = rule.ID()
				}
				continue
			}
			all = append(all, f)
		}
	}
	return all, firstErr, firstErrRuleID
}

// RunAllWithExecutions runs every rule this Registry contains -- identical
// findings/error behavior to RunAll, including RunAll's own documented
// first-error-only limitation above -- and additionally returns one
// findings.RuleExecutionRecord per rule ID in rules.AllRuleIDs(), the full
// rule universe this build knows about, not only the rules this particular
// registry happens to contain. This is the mechanism that lets a
// --manifests-only report's RuleExecutions explicitly mark the 29 rules
// NewManifestsOnlyRegistry excludes as not_applicable/not_evaluated,
// instead of those rules simply being absent with no trace.
//
// Per-rule-ID classification, for each ID in rules.AllRuleIDs():
//   - not registered in this registry at all -> Applicability:
//     not_applicable, State: not_evaluated.
//   - registered, and it's the one rule RunAll's firstErr came from ->
//     Applicability: applicable, State: failed, Reason from the error
//     (sanitized).
//   - registered, ran, and matches one of the small set of rules known to
//     silently skip on a specific missing collector key (see
//     ruleErrorsMapKeys in execution.go) -> Applicability: applicable,
//     State: insufficient_evidence.
//   - registered and ran otherwise -> Applicability: applicable, State:
//     evaluated.
//
// KNOWN LIMITATION (inherited from RunAll, not introduced here): because
// RunAll/runAll only ever preserves the FIRST rule's error, only that one
// rule (if any) can correctly be marked State: failed. Every other rule
// that also errored in the same scan is indistinguishable, from this
// method's vantage point, from a rule that ran cleanly and found nothing --
// it will incorrectly show State: evaluated with zero findings. This is
// documented, not silently swallowed: see RunAll's doc comment and
// docs/roadmap/v1.3.0-scope-audit.md Decision 5. Once RunAll's
// error-swallowing is fixed (an independent, out-of-scope patch), that fix
// should feed every rule's own error into this method's State: failed /
// Reason path, not just the first.
//
// insufficient_evidence coverage is similarly partial by design: only the 7
// rules that already have an explicit collector-Errors-map check are
// covered (6 here via ruleErrorsMapKeys, plus ADDON-002 which uses its own,
// different mechanism -- see execution.go). Retrofitting the other ~24
// rules is out of scope for v1.3.0 (deferred to v1.4.x per the locked scope
// document) -- they will show State: evaluated even on a genuine
// insufficient-evidence run.
//
// SCOPE BOUNDARY, deliberate, not a bug: the State: failed record this
// method constructs for the firstErr rule is real and present in this
// method's own return value (see execution_test.go's
// TestRunAllWithExecutions_FirstErrorMarkedFailed_SubsequentErrorSwallowed),
// but neither of today's two callers -- internal/cli/scan.go:280-283 and
// internal/cli/plan.go:270-273 -- ever surfaces it to a user. Both do:
//
//	fs, ruleExecutions, err := registry.RunAllWithExecutions(sc, targetVersion)
//	if err != nil {
//	    return fmt.Errorf("running rules: %w", err)
//	}
//
// and return right there, before findings.NewReportWithUpgradeContext is
// ever called -- so no *findings.Report is constructed, and therefore
// ruleExecutions (and the successfully-evaluated fs findings alongside it)
// are discarded with it. State: failed cannot appear in JSON, Markdown,
// HTML, Terminal, or Console output on this path; the command simply exits
// with a generic "running rules: ..." error.
//
// This is an intentional scope boundary of PR 1
// (docs/roadmap/v1.3.0-scope-audit.md's approved 8-PR sequence), confirmed
// by a dedicated verification pass: the locked scope document's PR 1
// acceptance criteria ("every scan... produces a complete 31-entry
// RuleExecutions array") and Capability 1's problem statement are both
// scoped to reports that actually get constructed; the document never
// discusses -- and Decision 5 explicitly defers a related, narrower
// question (RunAll's own first-error-only swallowing) as independently
// out-of-scope -- the larger question of emitting a partial, user-visible
// report when a rule errors and the command would otherwise abort. Making
// rule-error paths produce a partial report is a real, separate design
// question (which findings/coverage/exit-code semantics would a partial
// report carry?) deferred until it is explicitly scoped, not silently
// dropped. See TestRuleErrorAbortsBeforeAnyReportIsWritten
// (internal/cli/rule_error_scope_test.go) for a regression test protecting
// this as tested, intentional behavior.
func (r *Registry) RunAllWithExecutions(sc *ScanContext, targetVersion string) ([]findings.Finding, []findings.RuleExecutionRecord, error) {
	invoked := make(map[string]bool, len(r.rules))
	for _, rule := range r.rules {
		invoked[rule.ID()] = true
	}

	all, firstErr, firstErrRuleID := r.runAll(sc, targetVersion)

	universe := AllRuleIDs()
	records := make([]findings.RuleExecutionRecord, 0, len(universe))
	for _, ruleID := range universe {
		switch {
		case !invoked[ruleID]:
			records = append(records, findings.RuleExecutionRecord{
				RuleID:        ruleID,
				Applicability: findings.ApplicabilityNotApplicable,
				State:         findings.ExecutionNotEvaluated,
				Reason:        "not registered for this scan mode",
			})
		case firstErr != nil && ruleID == firstErrRuleID:
			records = append(records, findings.RuleExecutionRecord{
				RuleID:        ruleID,
				Applicability: findings.ApplicabilityApplicable,
				State:         findings.ExecutionFailed,
				Reason:        sanitizeRuleError(firstErr),
			})
		default:
			if reason, insufficient := insufficientEvidenceReason(ruleID, sc); insufficient {
				records = append(records, findings.RuleExecutionRecord{
					RuleID:        ruleID,
					Applicability: findings.ApplicabilityApplicable,
					State:         findings.ExecutionInsufficientEvidence,
					Reason:        reason,
				})
				continue
			}
			records = append(records, findings.RuleExecutionRecord{
				RuleID:        ruleID,
				Applicability: findings.ApplicabilityApplicable,
				State:         findings.ExecutionEvaluated,
			})
		}
	}
	return all, records, firstErr
}

// sanitizeRuleError renders err's text for a RuleExecutionRecord.Reason,
// matching the same trim-and-fallback hygiene already established by
// internal/cli/collection_issue.go's safeCollectionError: rule errors in
// this codebase are always constructed from static, operator-facing message
// strings (never directly from AWS/Kubernetes client credentials or raw
// secret material), so no further redaction beyond whitespace-trimming and
// a non-empty fallback is required here.
func sanitizeRuleError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return "rule returned an error with no message"
	}
	return msg
}
