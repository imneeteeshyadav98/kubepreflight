package report

import (
	"fmt"
	"sort"
	"strings"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
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

type findingViewMeta struct {
	Finding       findings.Finding
	ResourceLabel string
	ResourceKeys  []string
}

type reportFindingIndex struct {
	metas    []findingViewMeta
	all      []int
	blockers []int
	warnings []int
	infos    []int
}

func newReportFindingIndex(fs []findings.Finding) *reportFindingIndex {
	idx := &reportFindingIndex{
		metas: make([]findingViewMeta, len(fs)),
		all:   make([]int, len(fs)),
	}
	for i, f := range fs {
		label, keys := findingResourceIdentity(f)
		idx.metas[i] = findingViewMeta{Finding: f, ResourceLabel: label, ResourceKeys: keys}
		idx.all[i] = i
	}
	sort.Slice(idx.all, func(i, j int) bool {
		return findingMetaLess(idx.metas[idx.all[i]], idx.metas[idx.all[j]])
	})
	for _, pos := range idx.all {
		f := idx.metas[pos].Finding
		if f.EffectiveUpgradeGate() == findings.UpgradeGateBlock {
			idx.blockers = append(idx.blockers, pos)
			continue
		}
		if f.EffectiveUpgradeGate() == findings.UpgradeGateOperatorDecision || f.Severity == findings.SeverityWarning {
			idx.warnings = append(idx.warnings, pos)
			continue
		}
		if f.Severity == findings.SeverityInfo {
			idx.infos = append(idx.infos, pos)
		}
	}
	return idx
}

func (idx *reportFindingIndex) severity(sev findings.Severity) []findings.Finding {
	switch sev {
	case findings.SeverityBlocker:
		return idx.findings(idx.blockers)
	case findings.SeverityWarning:
		return idx.findings(idx.warnings)
	case findings.SeverityInfo:
		return idx.findings(idx.infos)
	default:
		return nil
	}
}

func (idx *reportFindingIndex) findings(positions []int) []findings.Finding {
	out := make([]findings.Finding, len(positions))
	for i, pos := range positions {
		out[i] = idx.metas[pos].Finding
	}
	return out
}

func (idx *reportFindingIndex) metasFor(positions []int) []findingViewMeta {
	out := make([]findingViewMeta, len(positions))
	for i, pos := range positions {
		out[i] = idx.metas[pos]
	}
	return out
}

func (idx *reportFindingIndex) allMetas() []findingViewMeta {
	return idx.metasFor(idx.all)
}

func (idx *reportFindingIndex) topMetas(limit int) []findingViewMeta {
	if limit > len(idx.all) {
		limit = len(idx.all)
	}
	if limit <= 0 {
		return nil
	}
	return idx.metasFor(idx.all[:limit])
}

func (idx *reportFindingIndex) actionableMetas() []findingViewMeta {
	out := make([]findingViewMeta, 0, len(idx.blockers)+len(idx.warnings))
	out = append(out, idx.metasFor(idx.blockers)...)
	out = append(out, idx.metasFor(idx.warnings)...)
	return out
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func impactScopesLabel(scopes []findings.ImpactScope) string {
	if len(scopes) == 0 {
		return "-"
	}
	parts := make([]string, len(scopes))
	for i, scope := range scopes {
		parts[i] = string(scope)
	}
	return strings.Join(parts, ", ")
}

// clusterDisplayName returns the short, human-friendly cluster identifier
// to show in report headings/titles, and the full identifier that should
// remain available (a tooltip, a copy button, the raw JSON) for anyone who
// needs the authoritative full value.
//
// Prefers EKSCluster.ClusterName when present — the exact --cluster-name
// value the operator passed, never derived from parsing anything — paired
// with EKSCluster.ARN (if AWS reported one) or ClusterContext as the full
// value. Otherwise falls back to ClusterContext, shortened only if it
// matches an EKS cluster ARN shape
// ("arn:aws:eks:<region>:<account>:cluster/<name>") — which is exactly
// what `aws eks update-kubeconfig` names a kubeconfig context by default,
// the real-world source of an unnecessarily wide cluster identifier this
// fixes. Any other ClusterContext shape (a hand-named context, a
// kind/minikube/on-prem context) is returned completely unchanged: this
// never blanks or guesses at a value it can't confidently parse.
func clusterDisplayName(r *findings.Report) (short, full string) {
	if r.EKSCluster != nil && r.EKSCluster.ClusterName != "" {
		full = r.EKSCluster.ARN
		if full == "" {
			full = r.ClusterContext
		}
		return r.EKSCluster.ClusterName, full
	}
	if name, ok := shortenEKSClusterARN(r.ClusterContext); ok {
		return name, r.ClusterContext
	}
	return r.ClusterContext, r.ClusterContext
}

// shortenEKSClusterARN extracts the cluster name from an EKS cluster ARN
// ("arn:aws:eks:<region>:<account-id>:cluster/<name>"), or reports
// ok=false for anything else so callers fall back to displaying the
// original value unchanged rather than guessing.
func shortenEKSClusterARN(identifier string) (name string, ok bool) {
	const prefix = "arn:aws:eks:"
	if !strings.HasPrefix(identifier, prefix) {
		return "", false
	}
	const marker = ":cluster/"
	idx := strings.LastIndex(identifier, marker)
	if idx == -1 {
		return "", false
	}
	name = identifier[idx+len(marker):]
	if name == "" {
		return "", false
	}
	return name, true
}

func yesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
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

// filterAndSort returns findings of the given severity, sorted Priority
// first, then Severity (uniform within this filtered slice, but the
// comparator stays shared with allSorted/topRisks for one sort order
// everywhere), then Confidence, then rule ID/resource name as a stable,
// diffable tie-break.
func filterAndSort(fs []findings.Finding, sev findings.Severity) []findings.Finding {
	return newReportFindingIndex(fs).severity(sev)
}

// allSorted returns every finding sorted Priority first (see findingLess),
// unmerged — used by the evidence appendix, which intentionally doesn't go
// through the Next Actions dedup.
func allSorted(fs []findings.Finding) []findings.Finding {
	idx := newReportFindingIndex(fs)
	return idx.findings(idx.all)
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
	metas := make([]findingViewMeta, len(fs))
	for i, f := range fs {
		label, keys := findingResourceIdentity(f)
		metas[i] = findingViewMeta{Finding: f, ResourceLabel: label, ResourceKeys: keys}
	}
	return buildNextActionsFromMetas(metas)
}

func buildNextActionsFromMetas(metas []findingViewMeta) []NextAction {
	if len(metas) == 0 {
		return nil
	}

	uf := newResourceUnionFind()
	findingKeys := make([][]string, len(metas))
	for i, meta := range metas {
		findingKeys[i] = meta.ResourceKeys
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

	groups := map[string][]findingViewMeta{}
	var order []string
	for i, meta := range metas {
		root := fmt.Sprintf("solo:%s", meta.Finding.Fingerprint) // no resources is unreachable (Finding.Validate requires >=1), guarded defensively
		if len(findingKeys[i]) > 0 {
			root = uf.find(findingKeys[i][0])
		}
		if _, seen := groups[root]; !seen {
			order = append(order, root)
		}
		groups[root] = append(groups[root], meta)
	}

	sort.Slice(order, func(i, j int) bool {
		gi, gj := groups[order[i]], groups[order[j]]
		pi, pj := groupMetaPriorityRank(gi), groupMetaPriorityRank(gj)
		if pi != pj {
			return pi < pj
		}
		si, sj := groupMetaSeverityRank(gi), groupMetaSeverityRank(gj)
		if si != sj {
			return si < sj
		}
		ci, cj := groupMetaConfidenceRank(gi), groupMetaConfidenceRank(gj)
		if ci != cj {
			return ci < cj
		}
		return groupMetaSortKey(gi) < groupMetaSortKey(gj)
	})

	actions := make([]NextAction, 0, len(order))
	for _, root := range order {
		group := groups[root]
		primaryMeta := primaryFindingMeta(group)
		primary := primaryMeta.Finding

		ruleIDs := make([]string, len(group))
		for j, meta := range group {
			ruleIDs[j] = meta.Finding.RuleID
		}
		sort.Strings(ruleIDs)

		resourceLabel := primaryMeta.ResourceLabel

		var related []findings.Finding
		for _, meta := range group {
			f := meta.Finding
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

// groupPriorityRank is the most urgent (lowest-rank) Priority among the
// group's findings — a group containing a P1 (global blocker) sorts
// first in Next Actions, ahead of even other Blockers, since fixing it
// may be a prerequisite for every other fix to actually take effect. This
// subsumes the old dedicated GlobalBlocker-first check: GlobalBlocker
// always maps to P1 (see findings.AssignPriority).
func groupPriorityRank(fs []findings.Finding) int {
	rank := findings.PriorityRank(string(findings.PriorityP4))
	for _, f := range fs {
		if r := findings.PriorityRank(f.Priority); r < rank {
			rank = r
		}
	}
	return rank
}

func groupMetaPriorityRank(metas []findingViewMeta) int {
	rank := findings.PriorityRank(string(findings.PriorityP4))
	for _, meta := range metas {
		if r := findings.PriorityRank(meta.Finding.Priority); r < rank {
			rank = r
		}
	}
	return rank
}

func groupConfidenceRank(fs []findings.Finding) int {
	rank := confidenceRank(findings.TierInferred)
	for _, f := range fs {
		if r := confidenceRank(f.Confidence); r < rank {
			rank = r
		}
	}
	return rank
}

func groupMetaConfidenceRank(metas []findingViewMeta) int {
	rank := confidenceRank(findings.TierInferred)
	for _, meta := range metas {
		if r := confidenceRank(meta.Finding.Confidence); r < rank {
			rank = r
		}
	}
	return rank
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

func groupMetaSeverityRank(metas []findingViewMeta) int {
	rank := severityRank(findings.SeverityInfo)
	for _, meta := range metas {
		if r := severityRank(meta.Finding.Severity); r < rank {
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

// confidenceRank orders ConfidenceTier for sorting — lower sorts first
// (most directly provable evidence first). Mirrors the display order
// confidenceMix (html.go) already uses.
func confidenceRank(c findings.ConfidenceTier) int {
	switch c {
	case findings.TierStaticCertain:
		return 0
	case findings.TierObserved:
		return 1
	case findings.TierProviderReported:
		return 2
	case findings.TierInferred:
		return 3
	default:
		return 4
	}
}

// findingLess is the shared sort order every renderer uses to rank
// findings: Priority first (P1 most urgent), Severity second, Confidence
// third, then RuleID/resource for a stable, diffable tie-break. Priority
// already reflects GlobalBlocker (see findings.AssignPriority) — a
// dedicated GlobalBlocker-first check on top of this would be redundant.
func findingLess(a, b findings.Finding) bool {
	labelA, _ := findingResourceIdentity(a)
	labelB, _ := findingResourceIdentity(b)
	return findingMetaLess(
		findingViewMeta{Finding: a, ResourceLabel: labelA},
		findingViewMeta{Finding: b, ResourceLabel: labelB},
	)
}

func findingMetaLess(a, b findingViewMeta) bool {
	if pa, pb := findings.PriorityRank(a.Finding.Priority), findings.PriorityRank(b.Finding.Priority); pa != pb {
		return pa < pb
	}
	if sa, sb := severityRank(a.Finding.Severity), severityRank(b.Finding.Severity); sa != sb {
		return sa < sb
	}
	if ca, cb := confidenceRank(a.Finding.Confidence), confidenceRank(b.Finding.Confidence); ca != cb {
		return ca < cb
	}
	if a.Finding.RuleID != b.Finding.RuleID {
		return a.Finding.RuleID < b.Finding.RuleID
	}
	return a.ResourceLabel < b.ResourceLabel
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

func primaryFindingMeta(metas []findingViewMeta) findingViewMeta {
	best := metas[0]
	bestRank := severityRank(best.Finding.Severity)
	for _, meta := range metas[1:] {
		r := severityRank(meta.Finding.Severity)
		if r < bestRank || (r == bestRank && meta.Finding.RuleID < best.Finding.RuleID) {
			best = meta
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

func groupMetaSortKey(metas []findingViewMeta) string {
	seen := map[string]bool{}
	var keys []string
	for _, meta := range metas {
		for _, k := range meta.ResourceKeys {
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
	label, _ := findingResourceIdentity(f)
	return label
}

func findingResourceIdentity(f findings.Finding) (string, []string) {
	refs, keys := uniqueConceptualRefsAndKeys(f.Resources)
	return resourceLabelFromRefs(refs), keys
}

func resourceLabelFromRefs(refs []findings.ResourceReference) string {
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
	out, _ := uniqueConceptualRefsAndKeys(refs)
	return out
}

func uniqueConceptualRefsAndKeys(refs []findings.ResourceReference) ([]findings.ResourceReference, []string) {
	var out []findings.ResourceReference
	keys := make([]string, 0, len(refs))
	seen := map[string]bool{}
	for _, ref := range refs {
		key, ok := ref.ConceptKey()
		if !ok {
			key = ref.OccurrenceKey()
		}
		if !seen[key] {
			seen[key] = true
			out = append(out, ref)
			keys = append(keys, key)
		}
	}
	return out, keys
}

func resourceLabel(r findings.ResourceReference) string {
	if r.Namespace != "" {
		return r.Kind + "/" + r.Namespace + "/" + r.Name
	}
	return r.Kind + "/" + r.Name
}
