package rules

import (
	"fmt"

	policyv1 "k8s.io/api/policy/v1"

	"kubepreflight/internal/findings"
)

// PDB001 flags a PodDisruptionBudget whose status.disruptionsAllowed is 0:
// the pods it selects can never be voluntarily evicted, which stalls node
// drain until the managed node group's ~15-minute eviction retry budget
// expires with PodEvictionFailure (deep dive Section 6, check PDB-001).
//
// This reads directly from PDB.Status (DisruptionsAllowed, CurrentHealthy,
// DesiredHealthy, ExpectedPods) rather than re-deriving replica health from
// Pods/Deployments: the PDB controller already computes these fields for
// every owning workload kind, so re-deriving them would be redundant and
// would silently miss owner kinds (e.g. StatefulSets) the collector doesn't
// list separately.
type PDB001 struct{}

func (PDB001) ID() string { return "PDB-001" }

func (PDB001) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	snap := sc.K8s
	var out []findings.Finding

	for _, pdb := range snap.PodDisruptionBudgets {
		if pdb.Status.DisruptionsAllowed > 0 {
			continue
		}
		out = append(out, pdb001Finding(pdb, targetVersion))
	}

	return out, nil
}

func pdb001Finding(pdb policyv1.PodDisruptionBudget, targetVersion string) findings.Finding {
	budget := "minAvailable: <unset>"
	if pdb.Spec.MinAvailable != nil {
		budget = fmt.Sprintf("minAvailable: %s", pdb.Spec.MinAvailable.String())
	} else if pdb.Spec.MaxUnavailable != nil {
		budget = fmt.Sprintf("maxUnavailable: %s", pdb.Spec.MaxUnavailable.String())
	}

	msg := fmt.Sprintf(
		"PodDisruptionBudget %s/%s: disruptionsAllowed=0 (%s, currentHealthy=%d, desiredHealthy=%d, expectedPods=%d) — matching pods cannot be voluntarily evicted, node drain will stall until the ~15-minute managed node group eviction budget expires",
		pdb.Namespace, pdb.Name, budget, pdb.Status.CurrentHealthy, pdb.Status.DesiredHealthy, pdb.Status.ExpectedPods)

	remediation := "Safest-first remediation ladder: (1) scale up replicas to create eviction headroom without changing the PDB contract; " +
		"(2) add topologySpreadConstraints to distribute the disruption cost across nodes; " +
		"(3) temporarily relax this PDB for the change window, with an explicit revert step in the change ticket. " +
		"Force-updating the node group to bypass PDBs is a last resort and must be a recorded business decision, not a default."

	ref := findings.LiveResource("PodDisruptionBudget", findings.ScopeNamespaced, pdb.Namespace, pdb.Name, string(pdb.UID))
	return findings.Finding{
		RuleID:     "PDB-001",
		Severity:   findings.SeverityBlocker,
		Confidence: findings.TierStaticCertain,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		Evidence: []string{
			"disruptionsAllowed: 0",
			budget,
			fmt.Sprintf("currentHealthy: %d", pdb.Status.CurrentHealthy),
			fmt.Sprintf("desiredHealthy: %d", pdb.Status.DesiredHealthy),
			fmt.Sprintf("expectedPods: %d", pdb.Status.ExpectedPods),
		},
		Remediation: remediation,
		Fingerprint: findings.FingerprintV2("PDB-001", targetVersion, "", ref),
	}
}
