package plan

// RuleProjectionPolicy classifies how honestly one rule ID's findings can
// be projected onto a hop beyond the immediate next one. The immediate
// next hop always gets a real, exact scan regardless of policy — this
// table only governs hops 2+.
type RuleProjectionPolicy int

const (
	// ProjectFromManifests: safe to re-evaluate exactly for any future hop
	// off the same manifest/CRD snapshot already collected for hop 1 — a
	// YAML file's apiVersion doesn't change based on how many cluster
	// upgrades have happened. Only applies to the manifest-plane half of a
	// rule's output (see the API-001 live-vs-manifest-plane split handled
	// by the caller in internal/cli/plan.go); the live-plane half of the
	// same rule ID is never safe to project and must be treated as
	// CarryForwardOnly regardless of this policy.
	ProjectFromManifests RuleProjectionPolicy = iota

	// ProjectFromFreshAWSQuery: requires a fresh, cheap, read-only AWS API
	// call per future hop's target version, but AWS's own API is
	// authoritative for whatever version it's asked about (unlike a
	// live-cluster-state assumption, which goes stale as the cluster
	// changes over the course of the upgrade). Only meaningful when
	// --provider=eks.
	ProjectFromFreshAWSQuery

	// CarryForwardOnly: describes current live-cluster state (node
	// versions, PDB overlap, webhook health, etc.) that will likely change
	// by the time the cluster actually reaches a future hop — nodes get
	// replaced, PDBs get fixed, webhooks get patched. Never projected as a
	// finding for hops beyond the first; surfaced only as a
	// CarryForwardNote explaining that a fresh scan is needed once that
	// hop is reached. This is also the fail-safe default for any rule ID
	// not explicitly listed below, so a newly added rule never silently
	// gets projected as an exact-sounding future finding without an
	// explicit, reviewed decision in this table.
	CarryForwardOnly
)

// RulePolicy maps every rule ID registered in rules.NewDefaultRegistry to
// its projection policy. internal/plan/classify_test.go asserts every
// currently-registered rule ID has an explicit entry here (not a
// fallthrough default), so adding a new rule without updating this table
// fails a test rather than silently under- or over-stating confidence in a
// plan's future hops.
var RulePolicy = map[string]RuleProjectionPolicy{
	"API-001":         ProjectFromManifests,
	"ADDON-001":       ProjectFromFreshAWSQuery,
	"EKS-NG-001":      CarryForwardOnly,
	"EKS-NG-002":      CarryForwardOnly,
	"EKS-NG-003":      CarryForwardOnly,
	"EKS-NG-004":      CarryForwardOnly,
	"EKS-INSIGHT-001": ProjectFromFreshAWSQuery,
	"EKS-INSIGHT-002": ProjectFromFreshAWSQuery,
	"EKS-INSIGHT-003": ProjectFromFreshAWSQuery,
	"WH-001":          CarryForwardOnly,
	"WH-002":          CarryForwardOnly,
	"WORKLOAD-001":    CarryForwardOnly,
	"PDB-001":         CarryForwardOnly,
	"PDB-002":         CarryForwardOnly,
	"NODE-001":        CarryForwardOnly,
	"NODE-002":        CarryForwardOnly,
	"NET-002":         CarryForwardOnly,
	"COREDNS-001":     CarryForwardOnly,
	"CRD-001":         CarryForwardOnly,
	"CRD-002":         CarryForwardOnly,
	"APISERVICE-001":  CarryForwardOnly,
}

// PolicyFor returns ruleID's projection policy, defaulting to
// CarryForwardOnly for any rule ID not explicitly listed in RulePolicy.
func PolicyFor(ruleID string) RuleProjectionPolicy {
	if policy, ok := RulePolicy[ruleID]; ok {
		return policy
	}
	return CarryForwardOnly
}
