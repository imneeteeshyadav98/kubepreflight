package rules

// NewDefaultRegistry returns a Registry pre-populated with every built-in
// check. New checks land here as they're implemented; nothing else in the
// scan pipeline needs to change to pick one up.
func NewDefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(API001{})
	r.Register(API002{})
	r.Register(WH001{})
	r.Register(WH002{})
	r.Register(WH004{})
	r.Register(WH005{})
	r.Register(DRAIN001{})
	r.Register(DRAIN002{})
	r.Register(DRAIN003{})
	r.Register(DRAIN004{})
	r.Register(DRAIN005{})
	r.Register(PDB001{})
	r.Register(PDB002{})
	r.Register(NODE001{})
	r.Register(NODE002{})
	r.Register(NODE003{})
	r.Register(NET002{})
	r.Register(WORKLOAD001{})
	r.Register(ADDON001{})
	r.Register(ADDON002{})
	r.Register(EKSNG001{})
	r.Register(EKSNG002{})
	r.Register(EKSNG003{})
	r.Register(EKSNG004{})
	r.Register(EKSINSIGHT001{})
	r.Register(EKSINSIGHT002{})
	r.Register(EKSINSIGHT003{})
	r.Register(COREDNS001{})
	r.Register(CRD001{})
	r.Register(CRD002{})
	r.Register(APIService001{})
	return r
}

// AllRuleIDs returns the canonical universe of every rule ID this build
// knows about -- i.e. every rule NewDefaultRegistry registers, in
// registration order. This is the "all 31 rules" list that
// Registry.RunAllWithExecutions (rule.go) diffs a specific registry's own
// RuleIDs() against, so a reduced registry (e.g. NewManifestsOnlyRegistry)
// can report which rules it excludes instead of those rules simply being
// absent with no trace. Building a full NewDefaultRegistry just to read its
// IDs is cheap -- every rule type here is a zero-size struct{}.
func AllRuleIDs() []string {
	return NewDefaultRegistry().RuleIDs()
}

// NewManifestsOnlyRegistry returns a Registry containing only the rules
// that read the Manifests plane (sc.Manifests) -- API-001 and API-002 are
// the only two today. Every other rule reads sc.K8s/sc.AWS directly and
// several (e.g. WH001, NODE001) don't nil-guard those fields, since a live
// cluster/AWS snapshot has always been mandatory up to now; running them
// against a nil sc.K8s would panic, not just report nothing. Used by
// `scan --manifests-only`, which deliberately skips all cluster/AWS
// collection.
func NewManifestsOnlyRegistry() *Registry {
	r := NewRegistry()
	r.Register(API001{})
	r.Register(API002{})
	return r
}
