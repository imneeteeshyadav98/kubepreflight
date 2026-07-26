package rules

import (
	"sort"
	"strings"
)

// ruleErrorsMapKeys records, for the handful of rules that already contain
// an explicit "skip this rule entirely if this specific collector key
// errored" guard, which Errors-map key(s) that guard checks and which plane
// (sc.K8s or sc.AWS) it reads from. This exists ONLY so
// Registry.RunAllWithExecutions (rule.go) can distinguish "the rule took
// its own skip-on-missing-evidence branch" from "the rule ran to
// completion and genuinely found nothing" -- both produce an identical
// (nil, nil) from Rule.Evaluate, so there is no way to tell them apart from
// outside the rule without mirroring the same keys each rule already checks
// internally.
//
// Scope, per docs/roadmap/v1.3.0-scope-audit.md's rule-evaluation inventory
// ("Rule evaluation inventory" table and "Aggregate counts"): exactly these
// 6 rules have this "silently skip the whole rule" shape:
//   - WH-002       (wh002.go:37-39)  -- sc.K8s.Errors["endpointslices"]
//   - PDB-001      (pdb001.go:33-35) -- sc.K8s.Errors["poddisruptionbudgets"]
//   - PDB-002      (pdb002.go:29-34) -- sc.K8s.Errors["poddisruptionbudgets"], sc.K8s.Errors["pods"]
//   - CRD-001      (crd001.go:22-24) -- sc.K8s.Errors["customresourcedefinitions"]
//   - CRD-002      (crd002.go:44-49) -- sc.K8s.Errors["customresourcedefinitions"], sc.K8s.Errors["endpointslices"]
//   - APISERVICE-001 (apiservice001.go:17-19) -- sc.K8s.Errors["apiservices"]
//
// A 7th rule, ADDON-002, also checks a collector-Errors key
// (addon001.go:123, addonVerificationError) but has a DISTINCT mechanism:
// instead of silently skipping, it turns the unverifiable state into a
// visible Warning finding (addon002Finding). Since that already produces a
// real, filterable finding rather than nothing, ADDON-002 is intentionally
// excluded from this map -- it is correctly left as a normal State:
// evaluated rule (it did evaluate; it just expressed uncertainty through an
// actual finding, matching the release's own locked invariant that
// rule-level failure/uncertainty stays distinct from a product finding
// "unless a rule intentionally emits an unverifiable finding" -- exactly
// the ADDON-002 pattern).
//
// The other ~24 rules have no Errors-map check at all -- a partial
// collector failure for their specific resource is completely
// indistinguishable, in this PR, from "evaluated cleanly, no problem
// found." Retrofitting them is explicitly out of scope for v1.3.0 (deferred
// to v1.4.x per the locked scope document's Capability 1 "Non-goals") --
// they are deliberately left as State: evaluated here, not silently
// claimed as fully covered.
//
// KNOWN FRAGILITY, accepted for v1.3.0: this list is a manually maintained
// mirror of each rule's own internal check, not derived from it via a
// shared code path. If a rule's own Errors-map key(s) ever change without
// updating this list, insufficient_evidence detection silently drifts out
// of sync. The alternative -- changing the Rule interface's Evaluate
// signature so all 31 rules report their own execution state directly --
// is a materially larger change explicitly out of this PR's scope (the
// locked document only asks for registry-level plumbing, not a Rule
// interface change).
var ruleErrorsMapKeys = map[string]struct {
	k8sKeys []string
	awsKeys []string
}{
	"WH-002":         {k8sKeys: []string{"endpointslices"}},
	"PDB-001":        {k8sKeys: []string{"poddisruptionbudgets"}},
	"PDB-002":        {k8sKeys: []string{"poddisruptionbudgets", "pods"}},
	"CRD-001":        {k8sKeys: []string{"customresourcedefinitions"}},
	"CRD-002":        {k8sKeys: []string{"customresourcedefinitions", "endpointslices"}},
	"APISERVICE-001": {k8sKeys: []string{"apiservices"}},
}

// insufficientEvidenceReason reports whether ruleID is one of the 6 rules
// in ruleErrorsMapKeys and, if so, whether the specific collector key(s) it
// depends on are present in sc's Errors maps -- i.e. whether this rule, per
// its own source, took its skip-on-missing-evidence branch this scan rather
// than its ran-and-found-nothing branch. Returns ok=false for every rule
// not in ruleErrorsMapKeys (the ~24 rules with no such check, plus
// ADDON-002 -- see ruleErrorsMapKeys's own comment) and for a rule in the
// map whose keys are all clean this scan.
func insufficientEvidenceReason(ruleID string, sc *ScanContext) (reason string, ok bool) {
	spec, tracked := ruleErrorsMapKeys[ruleID]
	if !tracked || sc == nil {
		return "", false
	}
	var missing []string
	if sc.K8s != nil {
		for _, key := range spec.k8sKeys {
			if _, errored := sc.K8s.Errors[key]; errored {
				missing = append(missing, key)
			}
		}
	}
	if sc.AWS != nil {
		for _, key := range spec.awsKeys {
			if _, errored := sc.AWS.Errors[key]; errored {
				missing = append(missing, key)
			}
		}
	}
	if len(missing) == 0 {
		return "", false
	}
	sort.Strings(missing)
	return "required collector data was unavailable for: " + strings.Join(missing, ", "), true
}
