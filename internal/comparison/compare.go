package comparison

import (
	"fmt"
	"strconv"

	"kubepreflight/internal/findings"
)

// Compare diffs baseline against current by Fingerprint -- never by
// message text, remediation text, or array position, all of which can
// change without the underlying issue changing (a remediation wording
// tweak between kubepreflight versions must never look like a resolved
// finding). Both reports must already be normalized (see
// LoadAndNormalize) so Priority/CanUpgradeContinue/UpgradeReadiness are
// populated regardless of which schema version produced them.
func Compare(baseline, current *findings.Report) (*Comparison, error) {
	baselineByFP, err := indexByFingerprint(baseline.Findings)
	if err != nil {
		return nil, fmt.Errorf("baseline: %w", err)
	}
	currentByFP, err := indexByFingerprint(current.Findings)
	if err != nil {
		return nil, fmt.Errorf("current: %w", err)
	}

	c := &Comparison{SchemaVersion: SchemaVersion}

	if baseline.TargetVersion != current.TargetVersion {
		c.Warnings = append(c.Warnings, fmt.Sprintf(
			"baseline was scanned at target-version %q and current at %q -- fingerprints are scoped to target version, so genuinely unchanged findings will show up as a new+resolved pair instead of unchanged. Re-scan both at the same target version for an accurate diff.",
			baseline.TargetVersion, current.TargetVersion))
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
	for fp, bf := range baselineByFP {
		if _, ok := currentByFP[fp]; !ok {
			c.Resolved = append(c.Resolved, Entry{bf})
		}
	}

	sortComparison(c)
	c.Summary = buildSummary(baseline, current, c)
	return c, nil
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
		BaselineVerdict: verdictOf(baseline),
		CurrentVerdict:  verdictOf(current),
		New:             len(c.New),
		Resolved:        len(c.Resolved),
		Changed:         len(c.Changed),
		Unchanged:       len(c.Unchanged),
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
		if e.Severity == findings.SeverityBlocker {
			s.NewBlockers++
		}
	}
	for _, e := range c.Resolved {
		if e.Severity == findings.SeverityBlocker {
			s.ResolvedBlockers++
		}
	}
	return s
}

// verdictOf reuses Report.Result() verbatim -- the same deterministic
// verdict logic scan/plan already drive their exit codes from. Comparison
// never reimplements or reinterprets that decision.
func verdictOf(r *findings.Report) string {
	return r.Result()
}
