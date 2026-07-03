package report

import (
	"sort"
	"strings"

	"kubepreflight/internal/findings"
)

// NextAction is one merged, resource-scoped action item: Primary's
// remediation is the instruction to follow, and Related holds any other
// finding on the same resource whose remediation text actually differs
// from Primary's (surfaced as a pointer, not repeated in full). Shared by
// every renderer (terminal, Markdown, HTML) so the dedup logic exists in
// exactly one place.
type NextAction struct {
	ResourceLabel string
	RuleIDs       []string
	Severity      findings.Severity
	Primary       findings.Finding
	Related       []findings.Finding
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// filterAndSort returns findings of the given severity, sorted by rule ID
// then resource name so repeated scans of the same cluster diff cleanly.
func filterAndSort(fs []findings.Finding, sev findings.Severity) []findings.Finding {
	var out []findings.Finding
	for _, f := range fs {
		if f.Severity == sev {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RuleID != out[j].RuleID {
			return out[i].RuleID < out[j].RuleID
		}
		return out[i].Resource.Name < out[j].Resource.Name
	})
	return out
}

// allSorted returns every finding sorted by rule ID then resource name,
// unmerged — used by the evidence appendix, which intentionally doesn't go
// through the Next Actions dedup.
func allSorted(fs []findings.Finding) []findings.Finding {
	out := make([]findings.Finding, len(fs))
	copy(out, fs)
	sort.Slice(out, func(i, j int) bool {
		if out[i].RuleID != out[j].RuleID {
			return out[i].RuleID < out[j].RuleID
		}
		return out[i].Resource.Name < out[j].Resource.Name
	})
	return out
}

// resourceKey identifies "the same real-world object" across findings from
// different rules, so Next Actions can merge them into one item. This is a
// literal Kind+Namespace+Name string match, not semantic resource
// resolution — known limitation for synthetic multi-resource findings like
// PDB-002's combined name, tracked for the Week 5 fingerprint-merge work
// rather than fixed here.
type resourceKey struct {
	Kind      string
	Namespace string
	Name      string
}

// buildNextActions groups findings by resource and returns one NextAction
// per resource: the highest-severity finding (ties broken by rule ID) is
// Primary, and any other finding in the group whose remediation text
// actually differs from Primary's ends up in Related. This is what
// prevents WH-001 and WH-002 firing on the same webhook from reading as
// two separate, potentially contradictory action items.
func buildNextActions(fs []findings.Finding) []NextAction {
	if len(fs) == 0 {
		return nil
	}

	groups := map[resourceKey][]findings.Finding{}
	var order []resourceKey
	for _, f := range fs {
		k := resourceKey{Kind: f.Resource.Kind, Namespace: f.Resource.Namespace, Name: f.Resource.Name}
		if _, seen := groups[k]; !seen {
			order = append(order, k)
		}
		groups[k] = append(groups[k], f)
	}

	sort.Slice(order, func(i, j int) bool {
		gi, gj := groups[order[i]], groups[order[j]]
		si, sj := groupSeverityRank(gi), groupSeverityRank(gj)
		if si != sj {
			return si < sj
		}
		if order[i].Kind != order[j].Kind {
			return order[i].Kind < order[j].Kind
		}
		if order[i].Namespace != order[j].Namespace {
			return order[i].Namespace < order[j].Namespace
		}
		return order[i].Name < order[j].Name
	})

	actions := make([]NextAction, 0, len(order))
	for _, k := range order {
		group := groups[k]
		primary := primaryFinding(group)

		ruleIDs := make([]string, len(group))
		for j, f := range group {
			ruleIDs[j] = f.RuleID
		}
		sort.Strings(ruleIDs)

		resourceLabel := k.Kind + "/" + k.Name
		if k.Namespace != "" {
			resourceLabel = k.Kind + "/" + k.Namespace + "/" + k.Name
		}

		var related []findings.Finding
		for _, f := range group {
			if f.RuleID == primary.RuleID || f.Remediation == primary.Remediation {
				continue
			}
			related = append(related, f)
		}

		actions = append(actions, NextAction{
			ResourceLabel: resourceLabel,
			RuleIDs:       ruleIDs,
			Severity:      primary.Severity,
			Primary:       primary,
			Related:       related,
		})
	}
	return actions
}

func groupSeverityRank(fs []findings.Finding) int {
	rank := severityRank(findings.SeverityInfo)
	for _, f := range fs {
		if r := severityRank(f.Severity); r < rank {
			rank = r
		}
	}
	return rank
}

func severityRank(s findings.Severity) int {
	switch s {
	case findings.SeverityBlocker:
		return 0
	case findings.SeverityWarning:
		return 1
	default:
		return 2
	}
}

func primaryFinding(fs []findings.Finding) findings.Finding {
	best := fs[0]
	bestRank := severityRank(best.Severity)
	for _, f := range fs[1:] {
		r := severityRank(f.Severity)
		if r < bestRank || (r == bestRank && f.RuleID < best.RuleID) {
			best = f
			bestRank = r
		}
	}
	return best
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i] + " ..."
	}
	return s
}

func resourceLabel(r findings.Resource) string {
	if r.Namespace != "" {
		return r.Kind + "/" + r.Namespace + "/" + r.Name
	}
	return r.Kind + "/" + r.Name
}
