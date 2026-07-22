package findings

// Priority classifies a finding's operational urgency for a Senior SRE
// planning an upgrade — a different axis than Severity (which governs the
// GO/NO-GO decision and exit code) and Confidence (which governs how
// certain the evidence is). Two Blocker-severity findings can carry very
// different priority: a global admission-webhook outage (P1) needs
// attention right now, while an incompatible EKS add-on (P4) needs fixing
// before starting the upgrade but isn't actively breaking anything yet.
type Priority string

const (
	// PriorityP1 is a global API write blocker: something that can break
	// kubectl apply/patch/scale, Helm upgrades, and controller
	// reconciliation cluster-wide — including the commands needed to fix
	// every other finding. Always wins regardless of rule ID: see
	// AssignPriority's GlobalBlocker override.
	PriorityP1 Priority = "P1"
	// PriorityP2 is a removed API or an EKS-reported critical upgrade
	// risk: a resource or behavior that may fail once the target
	// Kubernetes upgrade actually happens.
	PriorityP2 Priority = "P2"
	// PriorityP3 is a node-drain risk: something that can cause a node
	// drain to stall or fail during maintenance or a managed node group
	// upgrade.
	PriorityP3 Priority = "P3"
	// PriorityP4 is an unhealthy workload, add-on, or node-readiness
	// risk: the upgrade shouldn't begin while these are unhealthy, but
	// they aren't an active blocker on their own.
	PriorityP4 Priority = "P4"
)

var priorityReasons = map[Priority]string{
	PriorityP1: "May block kubectl apply/patch/scale, Helm upgrades, and controller reconciliation.",
	PriorityP2: "Resource or behavior may fail after the target Kubernetes upgrade.",
	PriorityP3: "Node drain may fail during maintenance or a managed node group upgrade.",
	PriorityP4: "Upgrade should not begin while workloads, nodes, or critical add-ons are unhealthy.",
}

// priorityByRuleID is the default rule ID -> Priority mapping. Every rule
// registered in internal/rules must have an entry here — a rule ID absent
// from this map defaults to PriorityP4 in AssignPriority rather than
// panicking, but internal/findings/priority_test.go asserts every ID in
// internal/rules' registry has an explicit entry, so that default path
// should never actually be exercised in practice.
//
// The mapping is a judgment call beyond GlobalBlocker's dynamic P1
// override (see AssignPriority): rules whose condition means a resource
// or behavior will fail once the target version is actually running
// (removed/deprecated APIs, AWS's own critical upgrade signal, missing
// upgrade-time network/IP preconditions) are P2; rules about a node
// failing to safely drain during the upgrade's rolling maintenance are
// P3; everything else — a narrowly-scoped admission risk, an
// unhealthy node group, informational context — is P4: real, but
// something to stabilize before starting rather than an active blocker in
// its own right. Catalog-known incompatible add-ons are P2 because the
// target Kubernetes version may fail without the add-on upgrade.
var priorityByRuleID = map[string]Priority{
	"API-001":         PriorityP2,
	"EKS-INSIGHT-001": PriorityP2,
	"CRD-001":         PriorityP2,
	"CRD-002":         PriorityP2,
	"APISERVICE-001":  PriorityP2,
	"NET-002":         PriorityP2,
	"NODE-002":        PriorityP2,

	"API-002":  PriorityP4,
	"NODE-003": PriorityP3,

	"PDB-001":    PriorityP3,
	"PDB-002":    PriorityP3,
	"NODE-001":   PriorityP3,
	"EKS-NG-002": PriorityP3,
	"DRAIN-001":  PriorityP3,
	"DRAIN-002":  PriorityP3,
	"DRAIN-003":  PriorityP3,
	"DRAIN-004":  PriorityP3,
	"DRAIN-005":  PriorityP3,

	"WH-001":          PriorityP4,
	"WH-002":          PriorityP4,
	"WH-004":          PriorityP4,
	"WH-005":          PriorityP4,
	"WORKLOAD-001":    PriorityP4,
	"ADDON-001":       PriorityP2,
	"ADDON-002":       PriorityP3,
	"COREDNS-001":     PriorityP4,
	"EKS-NG-001":      PriorityP4,
	"EKS-NG-003":      PriorityP4,
	"EKS-NG-004":      PriorityP4,
	"EKS-INSIGHT-002": PriorityP4,
	"EKS-INSIGHT-003": PriorityP4,
}

// affectedScopeByRuleID gives each rule a best-effort AffectedScope —
// "global", "namespace", "workload", "node", or "addon" — describing what
// class of resource the finding's remediation actually touches. This is a
// static per-rule classification, not derived from the finding's own
// Resources (which can reference several planes/kinds at once), so treat
// it as a coarse hint for grouping/filtering, not a precise resource
// inventory — Resources/ResourceLabel remain the authoritative detail.
var affectedScopeByRuleID = map[string]string{
	"EKS-INSIGHT-001": "global",
	"EKS-INSIGHT-002": "global",
	"EKS-INSIGHT-003": "global",
	"NET-002":         "global",

	"API-001":        "workload",
	"API-002":        "workload",
	"CRD-001":        "workload",
	"CRD-002":        "workload",
	"APISERVICE-001": "workload",
	"PDB-001":        "workload",
	"PDB-002":        "workload",
	"WH-001":         "workload",
	"WH-002":         "workload", // overridden to "global" when GlobalBlocker is true — see AssignPriority
	"WH-004":         "workload", // overridden to "global" when GlobalBlocker is true — see AssignPriority
	"WH-005":         "workload", // overridden to "global"/"cluster" by GlobalBlocker/CriticalInfra — see AssignPriority
	"WORKLOAD-001":   "workload",
	"DRAIN-001":      "workload",
	"DRAIN-002":      "workload",
	"DRAIN-003":      "workload",
	"DRAIN-005":      "workload", // overridden to "cluster" by CriticalInfra — see AssignPriority

	"NODE-001":   "node",
	"NODE-003":   "workload",
	"NODE-002":   "node",
	"DRAIN-004":  "node",
	"EKS-NG-001": "node",
	"EKS-NG-002": "node",
	"EKS-NG-003": "node",
	"EKS-NG-004": "node",

	"ADDON-001":   "addon",
	"ADDON-002":   "addon",
	"COREDNS-001": "addon",
}

// HasExplicitPriorityMapping reports whether ruleID has its own entry in
// priorityByRuleID, as opposed to landing on AssignPriority's P4 fallback
// by default. Exported only for internal/findings/priority_registry_test.go,
// which cross-checks this against the live rule registry (internal/rules)
// so a newly registered rule can't silently ship without a deliberate
// priority decision.
func HasExplicitPriorityMapping(ruleID string) bool {
	_, ok := priorityByRuleID[ruleID]
	return ok
}

// priorityRank orders Priority for sorting — lower sorts first (P1 most
// urgent). Anything unrecognized (including the empty string, for a
// Finding built without going through AssignPriority) sorts last.
func priorityRank(p string) int {
	switch Priority(p) {
	case PriorityP1:
		return 0
	case PriorityP2:
		return 1
	case PriorityP3:
		return 2
	case PriorityP4:
		return 3
	default:
		return 4
	}
}

// PriorityRank exports priorityRank for other packages (internal/report,
// internal/cli) that need to sort findings by the same priority order
// without duplicating this table.
func PriorityRank(p string) int { return priorityRank(p) }

// AssignPriority returns f with Priority, PriorityReason, AffectedScope,
// and CanUpgradeContinue set. Called once per finding from NewReport, so
// every Report's findings always carry these fields — rules themselves
// never set them directly.
//
// Two fact-based overrides escalate past the per-rule default, checked in
// increasing strength so the stronger one wins when both are set:
//
//   - CriticalInfra escalates to at least P2 with cluster scope: the same
//     condition on ordinary application workloads vs. cluster-critical
//     infrastructure (CNI, DNS, kube-system components) carries very
//     different upgrade risk, and a static per-rule-ID map can't express
//     that split.
//   - GlobalBlocker always wins regardless of rule ID: a fail-closed
//     webhook that happens to match broadly enough to block API writes
//     cluster-wide is the single most urgent thing on the cluster no
//     matter which rule caught it, and it can also break the remediation
//     commands for every other finding.
func AssignPriority(f Finding) Finding {
	priority, ok := priorityByRuleID[f.RuleID]
	if !ok {
		priority = PriorityP4
	}
	scope := affectedScopeByRuleID[f.RuleID]

	if f.CriticalInfra && f.RuleID != "NODE-003" && priorityRank(string(priority)) > priorityRank(string(PriorityP2)) {
		priority = PriorityP2
		scope = "cluster"
	}
	if f.RuleID == "ADDON-002" && addon002UpgradeRecommended(f) {
		priority = PriorityP4
		scope = "addon"
	}
	if f.GlobalBlocker {
		priority = PriorityP1
		scope = "global"
	}

	f.Priority = string(priority)
	f.PriorityReason = priorityReasons[priority]
	f.AffectedScope = scope
	if f.UpgradeGate == "" {
		f.UpgradeGate = f.EffectiveUpgradeGate()
	}
	f.CanUpgradeContinue = f.UpgradeGate == UpgradeGateAllow && priority != PriorityP1
	return f
}

func addon002UpgradeRecommended(f Finding) bool {
	for _, evidence := range f.Evidence {
		if evidence == "compatibility status: upgrade recommended" {
			return true
		}
	}
	return false
}
