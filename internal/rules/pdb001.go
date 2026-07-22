package rules

import (
	"encoding/json"
	"fmt"

	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
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
	if snap == nil {
		return nil, nil
	}
	if _, unavailable := snap.Errors["poddisruptionbudgets"]; unavailable {
		return nil, nil
	}
	var out []findings.Finding

	for _, pdb := range snap.PodDisruptionBudgets {
		if pdb.Status.DisruptionsAllowed > 0 || pdb.Status.ExpectedPods == 0 || pdb.Status.ObservedGeneration != pdb.Generation {
			continue
		}
		if pdb.Spec.UnhealthyPodEvictionPolicy != nil && *pdb.Spec.UnhealthyPodEvictionPolicy == policyv1.AlwaysAllow && pdb.Status.CurrentHealthy == 0 {
			continue
		}
		out = append(out, pdb001Finding(pdb, targetVersion, scanUpgradeContext(sc)))
	}

	return out, nil
}

func pdb001Finding(pdb policyv1.PodDisruptionBudget, targetVersion string, upgradeContexts ...findings.UpgradeContext) findings.Finding {
	upgradeContext := findings.UpgradeContextUnspecified
	if len(upgradeContexts) > 0 {
		upgradeContext = upgradeContexts[0]
	}
	budget := "minAvailable: <unset>"
	if pdb.Spec.MinAvailable != nil {
		budget = fmt.Sprintf("minAvailable: %s", pdb.Spec.MinAvailable.String())
	} else if pdb.Spec.MaxUnavailable != nil {
		budget = fmt.Sprintf("maxUnavailable: %s", pdb.Spec.MaxUnavailable.String())
	}

	severity, gate := drainDependentGate(upgradeContext)
	msg := fmt.Sprintf(
		"PodDisruptionBudget %s/%s: disruptionsAllowed=0 (%s, currentHealthy=%d, desiredHealthy=%d, expectedPods=%d) — healthy matching pods cannot currently be voluntarily evicted, which can block pod eviction during node drain or worker-node rollout",
		pdb.Namespace, pdb.Name, budget, pdb.Status.CurrentHealthy, pdb.Status.DesiredHealthy, pdb.Status.ExpectedPods)

	remediation := "Safest-first remediation ladder: (1) scale up replicas to create eviction headroom without changing the PDB contract; " +
		"(2) add topologySpreadConstraints to distribute the disruption cost across nodes; " +
		"(3) temporarily relax this PDB for the change window, with an explicit revert step in the change ticket. " +
		"Force-updating the node group to bypass PDBs is a last resort and must be a recorded business decision, not a default."

	ref := findings.LiveResource("PodDisruptionBudget", findings.ScopeNamespaced, pdb.Namespace, pdb.Name, string(pdb.UID))
	return findings.Finding{
		RuleID:     "PDB-001",
		Severity:   severity,
		Confidence: findings.TierObserved,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		ImpactScopes: []findings.ImpactScope{
			findings.ImpactScopeNodeDrain,
			findings.ImpactScopeWorkerRollout,
		},
		UpgradeGate: gate,
		Evidence: []string{
			"disruptionsAllowed: 0",
			budget,
			fmt.Sprintf("currentHealthy: %d", pdb.Status.CurrentHealthy),
			fmt.Sprintf("desiredHealthy: %d", pdb.Status.DesiredHealthy),
			fmt.Sprintf("expectedPods: %d", pdb.Status.ExpectedPods),
			fmt.Sprintf("observedGeneration: %d (metadata.generation: %d)", pdb.Status.ObservedGeneration, pdb.Generation),
		},
		Remediation:       remediation,
		RemediationDetail: pdb001RemediationDetail(pdb),
		Fingerprint:       findings.FingerprintV2("PDB-001", targetVersion, "", ref),
	}
}

// pdb001RemediationDetail builds the structured "current vs required" data
// for the HTML report. The replicas change row is only included when
// MinAvailable is set to an absolute integer — a percentage-based or
// maxUnavailable-based budget can't honestly be reduced to one required
// replica count, so those cases keep only the disruptionsAllowed row.
func pdb001RemediationDetail(pdb policyv1.PodDisruptionBudget) *findings.RemediationDetail {
	changes := []findings.RemediationChange{
		{Field: "disruptionsAllowed", Current: "0", Required: ">= 1"},
	}
	return &findings.RemediationDetail{
		Changes: changes,
		SafeFix: &findings.RemediationAction{
			Label: "Safe fix",
			Steps: []string{
				"Scale up replicas to create eviction headroom without changing the PDB contract.",
				"Add topologySpreadConstraints to distribute the disruption cost across nodes.",
			},
			Command: fmt.Sprintf("kubectl get pdb %s -n %s -o yaml\nkubectl get pods -n %s --show-labels", shellQuote(pdb.Name), shellQuote(pdb.Namespace), shellQuote(pdb.Namespace)),
		},
		Emergency:      pdbEmergencyAction(pdb),
		VerifyCommand:  fmt.Sprintf("kubectl describe pdb %s -n %s", shellQuote(pdb.Name), shellQuote(pdb.Namespace)),
		ExpectedResult: "Allowed disruptions >= 1",
	}
}

func pdbEmergencyAction(pdb policyv1.PodDisruptionBudget) *findings.RemediationAction {
	if pdb.Spec.MinAvailable != nil {
		// minAvailable: 0 is always maximally permissive — a universally
		// safe full relaxation regardless of the current value.
		return pdbEmergencyPatchAction(pdb, "minAvailable", pdb.Spec.MinAvailable, 0)
	}
	if pdb.Spec.MaxUnavailable != nil {
		return pdbEmergencyMaxUnavailableAction(pdb)
	}
	return nil
}

// pdbEmergencyMaxUnavailableAction relaxes maxUnavailable to the PDB's own
// expectedPods count — every matching pod may be unavailable at once,
// which is always at least as permissive as any valid current absolute
// value, unlike a hardcoded constant that could silently be a no-op or
// actively tighten the budget mid-incident. Percentage-based values and a
// current value that's already at or above this safe ceiling both fall
// back to inspect-first guidance instead of a copy-ready patch, since
// neither can be safely turned into a guaranteed-more-permissive patch
// here.
func pdbEmergencyMaxUnavailableAction(pdb policyv1.PodDisruptionBudget) *findings.RemediationAction {
	current := pdb.Spec.MaxUnavailable
	inspectFirst := &findings.RemediationAction{
		Label: "Emergency workaround",
		Risky: true,
		Steps: []string{
			fmt.Sprintf("maxUnavailable is currently %s. Inspect the PDB and workload before changing it — a copy-ready patch here isn't guaranteed to relax the budget without knowing the intended replica count.", current.String()),
			"If you do relax it, revert immediately after the change window and record the change as a business decision.",
		},
		Command: fmt.Sprintf("kubectl get pdb %s -n %s -o yaml", shellQuote(pdb.Name), shellQuote(pdb.Namespace)),
	}
	if current.Type != intstr.Int {
		return inspectFirst
	}
	required := int(pdb.Status.ExpectedPods)
	if required <= current.IntValue() {
		return inspectFirst
	}
	return pdbEmergencyPatchAction(pdb, "maxUnavailable", current, required)
}

// pdbEmergencyInspectFirstAction is the fallback when a value can't be
// safely turned into a copy-ready JSON Patch — inspect the live object
// instead of guessing.
func pdbEmergencyInspectFirstAction(pdb policyv1.PodDisruptionBudget, field string) *findings.RemediationAction {
	return &findings.RemediationAction{
		Label: "Emergency workaround",
		Risky: true,
		Steps: []string{
			fmt.Sprintf("%s could not be safely represented as a JSON Patch value. Inspect the PDB before changing it.", field),
			"If you do relax it, revert immediately after the change window and record the change as a business decision.",
		},
		Command: fmt.Sprintf("kubectl get pdb %s -n %s -o yaml", shellQuote(pdb.Name), shellQuote(pdb.Namespace)),
	}
}

// pdbEmergencyPatchAction pairs every relax/revert command with a JSON
// Patch "test" precondition against the value observed at scan time, so
// both commands are atomic and fail closed — rather than blindly
// overwriting — if the PDB was modified by someone else between scan time
// and copy-paste time. Without this, a bare "replace" can silently clobber
// a teammate's newer safety setting, and the paired "revert" can then
// restore a stale value instead of the one actually in effect.
func pdbEmergencyPatchAction(pdb policyv1.PodDisruptionBudget, field string, current *intstr.IntOrString, temporary int) *findings.RemediationAction {
	original, err := json.Marshal(current)
	if err != nil {
		return pdbEmergencyInspectFirstAction(pdb, field)
	}
	temp, err := json.Marshal(temporary)
	if err != nil {
		return pdbEmergencyInspectFirstAction(pdb, field)
	}
	return &findings.RemediationAction{
		Label: "Emergency workaround",
		Risky: true,
		Steps: []string{
			"Temporarily relax this PDB for the change window only — not a permanent fix.",
			"Both commands below include a JSON Patch precondition (\"test\") against the value observed at scan time — each fails closed with no change applied if the PDB has been modified since this scan. Re-run `kubectl get pdb ... -o yaml` and reassess before retrying.",
			"Revert immediately after the change window; record the change as a business decision.",
		},
		Command: fmt.Sprintf(
			"kubectl patch pdb %s -n %s --type=json -p='[{\"op\":\"test\",\"path\":\"/spec/%s\",\"value\":%s},{\"op\":\"replace\",\"path\":\"/spec/%s\",\"value\":%s}]'\n"+
				"# Revert immediately after the change window (fails closed if the value changed since the emergency patch):\n"+
				"kubectl patch pdb %s -n %s --type=json -p='[{\"op\":\"test\",\"path\":\"/spec/%s\",\"value\":%s},{\"op\":\"replace\",\"path\":\"/spec/%s\",\"value\":%s}]'",
			shellQuote(pdb.Name), shellQuote(pdb.Namespace), field, original, field, temp,
			shellQuote(pdb.Name), shellQuote(pdb.Namespace), field, temp, field, original,
		),
	}
}
