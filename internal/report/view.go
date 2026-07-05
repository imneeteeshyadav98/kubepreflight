package report

import (
	"fmt"
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

func coverageIssueLines(r *findings.Report) []string {
	planes := []struct {
		name     string
		coverage findings.PlaneCoverage
	}{{"Kubernetes", r.Coverage.Kubernetes}, {"AWS", r.Coverage.AWS}, {"Manifests", r.Coverage.Manifests}}
	var out []string
	for _, plane := range planes {
		if plane.coverage.Status != findings.CoveragePartial {
			continue
		}
		if len(plane.coverage.Errors) == 0 {
			out = append(out, plane.name+": incomplete")
			continue
		}
		for _, err := range plane.coverage.Errors {
			out = append(out, plane.name+": "+err)
		}
	}
	return out
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
		return findingResourceLabel(out[i]) < findingResourceLabel(out[j])
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
		return findingResourceLabel(out[i]) < findingResourceLabel(out[j])
	})
	return out
}

// buildNextActions groups findings by resource and returns one NextAction
// per resource: the highest-severity finding (ties broken by rule ID) is
// Primary, and any other finding in the group whose remediation text
// actually differs from Primary's ends up in Related. Grouping merges any
// findings that share at least one conceptual resource — via
// resourceUnionFind, not exact resource-set equality — so a root-cause
// chain (e.g. PDB-001 on a PDB, and PDB-002 on that same PDB plus a
// duplicate) reads as one action item. This is what prevents WH-001 and
// WH-002 firing on the same webhook from reading as two separate,
// potentially contradictory action items, generalized to any shared
// resource rather than only an identical resource set.
func buildNextActions(fs []findings.Finding) []NextAction {
	if len(fs) == 0 {
		return nil
	}

	uf := newResourceUnionFind()
	findingKeys := make([][]string, len(fs))
	for i, f := range fs {
		findingKeys[i] = individualResourceKeys(f)
		uf.unionAll(findingKeys[i])
	}
	firstCarrier := map[string]string{} // resource key -> that finding's keys[0], for cross-finding unioning
	for _, keys := range findingKeys {
		for _, k := range keys {
			if rep, ok := firstCarrier[k]; ok {
				uf.union(k, rep)
			} else {
				firstCarrier[k] = keys[0]
			}
		}
	}

	groups := map[string][]findings.Finding{}
	var order []string
	for i, f := range fs {
		root := fmt.Sprintf("solo:%s", f.Fingerprint) // no resources is unreachable (Finding.Validate requires >=1), guarded defensively
		if len(findingKeys[i]) > 0 {
			root = uf.find(findingKeys[i][0])
		}
		if _, seen := groups[root]; !seen {
			order = append(order, root)
		}
		groups[root] = append(groups[root], f)
	}

	sort.Slice(order, func(i, j int) bool {
		gi, gj := groups[order[i]], groups[order[j]]
		bi, bj := groupHasGlobalBlocker(gi), groupHasGlobalBlocker(gj)
		if bi != bj {
			return bi // global-blocker groups always sort first
		}
		si, sj := groupSeverityRank(gi), groupSeverityRank(gj)
		if si != sj {
			return si < sj
		}
		return groupSortKey(gi) < groupSortKey(gj)
	})

	actions := make([]NextAction, 0, len(order))
	for _, root := range order {
		group := groups[root]
		primary := primaryFinding(group)

		ruleIDs := make([]string, len(group))
		for j, f := range group {
			ruleIDs[j] = f.RuleID
		}
		sort.Strings(ruleIDs)

		resourceLabel := findingResourceLabel(primary)

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

// groupHasGlobalBlocker reports whether any finding in the group can block
// other remediation commands from succeeding at all (e.g. a fail-closed
// webhook with no healthy backend) — such groups sort first in Next
// Actions, ahead of even other Blockers, since fixing them may be a
// prerequisite for every other fix to actually take effect.
func groupHasGlobalBlocker(fs []findings.Finding) bool {
	for _, f := range fs {
		if f.GlobalBlocker {
			return true
		}
	}
	return false
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

// individualResourceKeys returns the finding's conceptual resource keys,
// unmerged (unlike the old findingResourceKey, which joined them into one
// string) — buildNextActions unions these individually so two findings
// that share even one resource merge into one group.
func individualResourceKeys(f findings.Finding) []string {
	keys := make([]string, 0, len(f.Resources))
	seen := map[string]bool{}
	for _, ref := range f.Resources {
		key, ok := ref.ConceptKey()
		if !ok {
			key = ref.OccurrenceKey()
		}
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	return keys
}

// groupSortKey gives a deterministic, resource-derived tie-break string for
// a merged group of findings — the sorted union of every member's resource
// keys, the same shape the old single-finding resourceKey had.
func groupSortKey(fs []findings.Finding) string {
	seen := map[string]bool{}
	var keys []string
	for _, f := range fs {
		for _, k := range individualResourceKeys(f) {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	sort.Strings(keys)
	return strings.Join(keys, ":")
}

// resourceUnionFind is a disjoint-set over resource keys: two findings
// merge into the same Next Action group when they share at least one
// resource key, even if their full resource sets differ. This is a
// superset of "identical resource set" grouping, so it also merges chains
// like PDB-001 on a PDB plus PDB-002 on that same PDB and a duplicate.
type resourceUnionFind struct {
	parent map[string]string
}

func newResourceUnionFind() *resourceUnionFind {
	return &resourceUnionFind{parent: map[string]string{}}
}

func (u *resourceUnionFind) find(x string) string {
	if _, ok := u.parent[x]; !ok {
		u.parent[x] = x
		return x
	}
	if u.parent[x] != x {
		u.parent[x] = u.find(u.parent[x])
	}
	return u.parent[x]
}

func (u *resourceUnionFind) union(a, b string) {
	ra, rb := u.find(a), u.find(b)
	if ra != rb {
		u.parent[ra] = rb
	}
}

func (u *resourceUnionFind) unionAll(keys []string) {
	for i := 1; i < len(keys); i++ {
		u.union(keys[0], keys[i])
	}
}

func findingResourceLabel(f findings.Finding) string {
	refs := uniqueConceptualRefs(f.Resources)
	if len(refs) == 0 {
		return "Unknown/-"
	}
	if len(refs) == 1 {
		return resourceLabel(refs[0])
	}

	kind, namespace := refs[0].Kind, refs[0].Namespace
	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref.Kind != kind || ref.Namespace != namespace {
			labels := make([]string, 0, len(refs))
			for _, item := range refs {
				labels = append(labels, resourceLabel(item))
			}
			sort.Strings(labels)
			return strings.Join(labels, ", ")
		}
		names = append(names, ref.Name)
	}
	sort.Strings(names)
	if namespace != "" {
		return kind + "/" + namespace + "/" + strings.Join(names, ",")
	}
	return kind + "/" + strings.Join(names, ",")
}

func uniqueConceptualRefs(refs []findings.ResourceReference) []findings.ResourceReference {
	var out []findings.ResourceReference
	seen := map[string]bool{}
	for _, ref := range refs {
		key, ok := ref.ConceptKey()
		if !ok {
			key = ref.OccurrenceKey()
		}
		if !seen[key] {
			seen[key] = true
			out = append(out, ref)
		}
	}
	return out
}

func resourceLabel(r findings.ResourceReference) string {
	if r.Namespace != "" {
		return r.Kind + "/" + r.Namespace + "/" + r.Name
	}
	return r.Kind + "/" + r.Name
}
