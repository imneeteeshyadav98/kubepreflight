package comparison

import (
	"fmt"
	"strconv"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

// Compare diffs baseline against current by Fingerprint -- never by
// message text, remediation text, or array position, all of which can
// change without the underlying issue changing (a remediation wording
// tweak between kubepreflight versions must never look like a resolved
// finding). Both reports must already be normalized (see
// LoadAndNormalize) so Priority/CanUpgradeContinue/UpgradeReadiness are
// populated regardless of which schema version produced them.
//
// A baseline finding with no matching fingerprint in current is bucketed
// into either Resolved or NotReEvaluated based on whether the finding's rule
// can be PROVEN to have evaluated cleanly this scan -- never assumed. See
// ruleProvenEvaluated for the exact, conservative-by-default decision tree;
// this is the core safety invariant PR 5 exists to enforce (see
// docs/roadmap/v1.3.0-scope-audit.md's "Lock semantic rules" section): a
// finding's absence from the current report is resolution evidence only
// when its rule demonstrably ran and found nothing, never merely because
// the finding itself is missing.
func Compare(baseline, current *findings.Report) (*Comparison, error) {
	baselineByFP, err := indexByFingerprint(baseline.Findings)
	if err != nil {
		return nil, fmt.Errorf("baseline: %w", err)
	}
	currentByFP, err := indexByFingerprint(current.Findings)
	if err != nil {
		return nil, fmt.Errorf("current: %w", err)
	}

	// New/Resolved/Changed/Unchanged start as empty slices, not nil --
	// encoding/json marshals a nil slice as `null`, and the most common
	// comparison result (nothing changed) would otherwise serialize every
	// bucket as `null` instead of `[]`, breaking any consumer (this
	// package's own JSON schema, the Console, or a shell script doing
	// `jq '.new[]'`) that reasonably expects an array it can iterate
	// unconditionally.
	c := &Comparison{
		SchemaVersion:  SchemaVersion,
		New:            []Entry{},
		Resolved:       []Entry{},
		Changed:        []Changed{},
		Unchanged:      []Entry{},
		NotReEvaluated: []Entry{},
	}

	if baseline.TargetVersion != current.TargetVersion {
		c.Warnings = append(c.Warnings, fmt.Sprintf(
			"baseline was scanned at target-version %q and current at %q -- fingerprints are scoped to target version, so genuinely unchanged findings will show up as a new+resolved pair instead of unchanged. Re-scan both at the same target version for an accurate diff.",
			baseline.TargetVersion, current.TargetVersion))
	}
	if baseline.UpgradeContext != current.UpgradeContext {
		c.Warnings = append(c.Warnings, fmt.Sprintf(
			"baseline used upgradeContext %q and current used %q -- blocker counts and verdicts are context-aware, so review gate changes with the selected operation in mind.",
			baseline.UpgradeContext, current.UpgradeContext))
	}

	for fp, cf := range currentByFP {
		bf, ok := baselineByFP[fp]
		if !ok {
			c.New = append(c.New, Entry{cf})
			continue
		}
		if changes := diffFinding(bf, cf); len(changes) > 0 {
			c.Changed = append(c.Changed, Changed{
				Fingerprint: fp,
				RuleID:      cf.RuleID,
				Resources:   cf.Resources,
				Changes:     changes,
			})
		} else {
			c.Unchanged = append(c.Unchanged, Entry{cf})
		}
	}
	// currentExecByRuleID is built once, outside the loop below, so
	// classifying every baseline-only finding is an O(1) map lookup rather
	// than an O(len(RuleExecutions)) scan per finding.
	currentExecByRuleID := indexRuleExecutions(current.RuleExecutions)
	for fp, bf := range baselineByFP {
		if _, ok := currentByFP[fp]; ok {
			continue
		}
		if ruleProvenEvaluated(bf.RuleID, currentExecByRuleID) {
			c.Resolved = append(c.Resolved, Entry{bf})
		} else {
			c.NotReEvaluated = append(c.NotReEvaluated, Entry{bf})
		}
	}

	sortComparison(c)
	c.Summary = buildSummary(baseline, current, c)
	return c, nil
}

// indexRuleExecutions builds a rule-ID-keyed lookup of current's
// rule-execution records for ruleProvenEvaluated. A rule ID absent from this
// map (including every rule ID when current.RuleExecutions is empty/nil --
// either a pre-v1.3.0 document that bypassed comparison.LoadAndNormalize
// entirely, or any other reason RuleExecutions ended up empty) simply
// doesn't appear as a key, so the caller's plain map lookup already
// conservatively treats "no record for this rule ID" and "no RuleExecutions
// data at all" identically -- both fail the ok check and fall through to
// ruleProvenEvaluated's false path. Lookups are exact rule-ID string
// matches only; there is deliberately no fuzzy/partial/prefix matching
// anywhere in this path.
func indexRuleExecutions(records []findings.RuleExecutionRecord) map[string]findings.RuleExecutionRecord {
	byRuleID := make(map[string]findings.RuleExecutionRecord, len(records))
	for _, rec := range records {
		byRuleID[rec.RuleID] = rec
	}
	return byRuleID
}

// ruleProvenEvaluated reports whether ruleID has proof, in current's own
// rule-execution records, that it evaluated cleanly this scan -- the only
// condition under which a baseline finding missing from current is
// classified Resolved rather than NotReEvaluated. Every other case returns
// false, conservatively, per the invariant locked in
// docs/roadmap/v1.3.0-scope-audit.md ("Lock semantic rules"):
//
//   - no record for ruleID at all (rule ID unknown to current's canonical
//     execution records, e.g. a renamed/removed historical rule ID) -> false
//   - Applicability == not_applicable -> false (the rule was excluded from
//     this scan's scope entirely; that's not evidence of a clean pass)
//   - State == not_evaluated, insufficient_evidence, or failed -> false
//   - Applicability == applicable AND State == evaluated -> true, the only
//     proof-of-resolution case
//
// byRuleID is expected to come from indexRuleExecutions(current.RuleExecutions)
// -- when current carries no RuleExecutions data at all, byRuleID is empty
// and every lookup here misses, so every baseline-only finding conservatively
// falls through to NotReEvaluated rather than being silently read as
// Resolved.
func ruleProvenEvaluated(ruleID string, byRuleID map[string]findings.RuleExecutionRecord) bool {
	rec, ok := byRuleID[ruleID]
	if !ok {
		return false
	}
	return rec.Applicability == findings.ApplicabilityApplicable && rec.State == findings.ExecutionEvaluated
}

func indexByFingerprint(fs []findings.Finding) (map[string]findings.Finding, error) {
	byFP := make(map[string]findings.Finding, len(fs))
	for _, f := range fs {
		if f.Fingerprint == "" {
			return nil, fmt.Errorf("finding %s has no fingerprint -- cannot compare without stable identity", f.RuleID)
		}
		if _, dup := byFP[f.Fingerprint]; dup {
			return nil, fmt.Errorf("duplicate fingerprint %q (rule %s) -- a findings.json document must not contain two findings with the same fingerprint", f.Fingerprint, f.RuleID)
		}
		byFP[f.Fingerprint] = f
	}
	return byFP, nil
}

// diffFinding compares only decision-relevant fields -- never Message,
// Evidence, or Remediation text, so a wording change between kubepreflight
// versions is never mistaken for the underlying issue actually changing.
func diffFinding(before, after findings.Finding) map[string]FieldChange {
	changes := map[string]FieldChange{}
	if before.Severity != after.Severity {
		changes["severity"] = FieldChange{Before: string(before.Severity), After: string(after.Severity)}
	}
	if before.Priority != after.Priority {
		changes["priority"] = FieldChange{Before: before.Priority, After: after.Priority}
	}
	if before.Confidence != after.Confidence {
		changes["confidence"] = FieldChange{Before: string(before.Confidence), After: string(after.Confidence)}
	}
	if before.CanUpgradeContinue != after.CanUpgradeContinue {
		changes["canUpgradeContinue"] = FieldChange{Before: strconv.FormatBool(before.CanUpgradeContinue), After: strconv.FormatBool(after.CanUpgradeContinue)}
	}
	if before.EffectiveUpgradeGate() != after.EffectiveUpgradeGate() {
		changes["upgradeGate"] = FieldChange{Before: string(before.EffectiveUpgradeGate()), After: string(after.EffectiveUpgradeGate())}
	}
	if impactScopeIdentity(before.ImpactScopes) != impactScopeIdentity(after.ImpactScopes) {
		changes["impactScopes"] = FieldChange{Before: impactScopeIdentity(before.ImpactScopes), After: impactScopeIdentity(after.ImpactScopes)}
	}
	if before.AffectedScope != after.AffectedScope {
		changes["affectedScope"] = FieldChange{Before: before.AffectedScope, After: after.AffectedScope}
	}
	// Defensive only -- Fingerprint already hashes ruleID and each
	// resource's concept key, so two findings sharing a fingerprint could
	// never actually differ here. Kept for correctness if that ever
	// changes, not because it's expected to fire.
	if before.RuleID != after.RuleID {
		changes["ruleId"] = FieldChange{Before: before.RuleID, After: after.RuleID}
	}
	if resourceIdentity(before.Resources) != resourceIdentity(after.Resources) {
		changes["resource"] = FieldChange{Before: resourceIdentity(before.Resources), After: resourceIdentity(after.Resources)}
	}
	return changes
}

func resourceIdentity(refs []findings.ResourceReference) string {
	s := ""
	for _, r := range refs {
		s += string(r.Plane) + ":" + r.Kind + ":" + r.Namespace + "/" + r.Name + ";"
	}
	return s
}

func buildSummary(baseline, current *findings.Report, c *Comparison) Summary {
	s := Summary{
		BaselineVerdict:        verdictOf(baseline),
		CurrentVerdict:         verdictOf(current),
		BaselineUpgradeContext: string(baseline.UpgradeContext),
		CurrentUpgradeContext:  string(current.UpgradeContext),
		New:                    len(c.New),
		Resolved:               len(c.Resolved),
		Changed:                len(c.Changed),
		Unchanged:              len(c.Unchanged),
		NotReEvaluated:         len(c.NotReEvaluated),
	}
	s.VerdictChanged = s.BaselineVerdict != s.CurrentVerdict
	if baseline.UpgradeReadiness != nil {
		s.BaselineReadinessScore = baseline.UpgradeReadiness.ReadinessScore
	}
	if current.UpgradeReadiness != nil {
		s.CurrentReadinessScore = current.UpgradeReadiness.ReadinessScore
	}
	s.ReadinessScoreDelta = s.CurrentReadinessScore - s.BaselineReadinessScore
	for _, e := range c.New {
		if e.EffectiveUpgradeGate() == findings.UpgradeGateBlock {
			s.NewBlockers++
		}
	}
	for _, e := range c.Resolved {
		if e.EffectiveUpgradeGate() == findings.UpgradeGateBlock {
			s.ResolvedBlockers++
		}
	}
	return s
}

func impactScopeIdentity(scopes []findings.ImpactScope) string {
	s := ""
	for _, scope := range scopes {
		s += string(scope) + ";"
	}
	return s
}

// verdictOf reuses Report.Result() verbatim -- the same deterministic
// verdict logic scan/plan already drive their exit codes from. Comparison
// never reimplements or reinterprets that decision.
func verdictOf(r *findings.Report) string {
	return r.Result()
}
