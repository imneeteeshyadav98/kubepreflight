package rules

import (
	"fmt"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"kubepreflight/internal/collectors/k8s"
	"kubepreflight/internal/collectors/manifest"
	"kubepreflight/internal/findings"
)

// DRAIN001 flags a Deployment or StatefulSet whose effective desired
// replica count is 1: when the node hosting its one pod drains (or the
// pod is otherwise evicted), the workload has zero available replicas
// until a replacement schedules and becomes Ready elsewhere -- unlike a
// multi-replica workload, where a drain only removes one of several
// available pods.
//
// This is a different fact than PDB-001: PDB-001 says "eviction can't
// even start" (disruptionsAllowed == 0); DRAIN-001 says "even if eviction
// succeeds cleanly, this workload has a capacity gap during the
// replacement window." A singleton with a healthy, non-zero PDB still has
// this gap -- the PDB only governs whether eviction is currently
// permitted, not whether the workload keeps serving during it. Per design
// decision, PDB presence/absence is evidence only and never escalates or
// suppresses this finding: a singleton without a PDB is not, by itself, a
// Blocker (that would duplicate PDB-001's job) and a singleton *with* a
// healthy PDB does not become safe (the PDB doesn't close this gap).
//
// DaemonSets are excluded: a DaemonSet's "one pod per node" is intentional
// design, not a singleton risk -- see DRAIN-005 for DaemonSet rollout
// health instead. Jobs/CronJobs are excluded: their single-completion
// semantics aren't a Deployment/StatefulSet's continuous-availability
// contract.
type DRAIN001 struct{}

func (DRAIN001) ID() string { return "DRAIN-001" }

func (DRAIN001) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	if sc == nil {
		return nil, nil
	}
	var out []findings.Finding

	if sc.K8s != nil {
		for _, d := range sc.K8s.Deployments {
			if d.DeletionTimestamp != nil {
				continue
			}
			if !isSingletonReplicaCount(d.Spec.Replicas) {
				continue
			}
			out = append(out, drain001DeploymentFinding(sc.K8s, d, targetVersion))
		}
		for _, sts := range sc.K8s.StatefulSets {
			if sts.DeletionTimestamp != nil {
				continue
			}
			if !isSingletonReplicaCount(sts.Spec.Replicas) {
				continue
			}
			out = append(out, drain001StatefulSetFinding(sc.K8s, sts, targetVersion))
		}
	}

	if sc.Manifests != nil {
		for _, obj := range sc.Manifests.Workloads {
			if obj.Kind != "Deployment" && obj.Kind != "StatefulSet" {
				continue
			}
			var replicas *int32
			if obj.Replicas != nil {
				r := int32(*obj.Replicas)
				replicas = &r
			}
			if !isSingletonReplicaCount(replicas) {
				continue
			}
			out = append(out, drain001ManifestFinding(obj, targetVersion))
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Fingerprint < out[j].Fingerprint
	})
	return out, nil
}

// isSingletonReplicaCount reports whether a Deployment/StatefulSet's
// desired replica count is effectively 1: either explicitly set to 1, or
// unset (both kinds default an unset spec.replicas to 1).
func isSingletonReplicaCount(replicas *int32) bool {
	return replicas == nil || *replicas == 1
}

func drain001DeploymentFinding(snap *k8s.Snapshot, d appsv1.Deployment, targetVersion string) findings.Finding {
	pods := matchingPodNames(snap, d.Namespace, d.Spec.Selector)
	pdbNames := matchingPDBNames(snap, d.Namespace, d.Spec.Selector)
	strategy := string(d.Spec.Strategy.Type)
	if strategy == "" {
		strategy = "RollingUpdate"
	}
	return drain001Finding(drain001Params{
		Kind: "Deployment", Namespace: d.Namespace, Name: d.Name, UID: string(d.UID),
		DesiredReplicas: 1, ReadyReplicas: d.Status.ReadyReplicas, AvailableReplicas: d.Status.AvailableReplicas,
		RolloutStrategy: strategy, PDBNames: pdbNames, AffectedPods: pods,
	}, targetVersion)
}

func drain001StatefulSetFinding(snap *k8s.Snapshot, sts appsv1.StatefulSet, targetVersion string) findings.Finding {
	pods := matchingPodNames(snap, sts.Namespace, sts.Spec.Selector)
	pdbNames := matchingPDBNames(snap, sts.Namespace, sts.Spec.Selector)
	strategy := string(sts.Spec.UpdateStrategy.Type)
	if strategy == "" {
		strategy = "RollingUpdate"
	}
	if sts.Spec.PodManagementPolicy != "" {
		strategy = fmt.Sprintf("%s (podManagementPolicy: %s)", strategy, sts.Spec.PodManagementPolicy)
	}
	return drain001Finding(drain001Params{
		Kind: "StatefulSet", Namespace: sts.Namespace, Name: sts.Name, UID: string(sts.UID),
		DesiredReplicas: 1, ReadyReplicas: sts.Status.ReadyReplicas, AvailableReplicas: sts.Status.AvailableReplicas,
		RolloutStrategy: strategy, PDBNames: pdbNames, AffectedPods: pods,
	}, targetVersion)
}

// isLiveWorkloadPod excludes pods that shouldn't count as "currently
// serving" evidence for a drain-readiness check: deleting (eviction
// already in flight) or in a terminal phase (Succeeded/Failed -- a
// completed Job-owned pod, for instance, must never be counted as an
// active replica of an unrelated selector match).
func isLiveWorkloadPod(pod corev1.Pod) bool {
	if pod.DeletionTimestamp != nil {
		return false
	}
	return pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed
}

// matchingPodNames returns the names of live pods in namespace matching
// selector, sorted for deterministic evidence output. A nil or
// unparseable selector matches nothing, rather than risking a
// match-everything false positive.
func matchingPodNames(snap *k8s.Snapshot, namespace string, selector *metav1.LabelSelector) []string {
	if selector == nil {
		return nil
	}
	sel, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil
	}
	var names []string
	for _, pod := range snap.Pods {
		if pod.Namespace != namespace || !isLiveWorkloadPod(pod) {
			continue
		}
		if sel.Matches(labels.Set(pod.Labels)) {
			names = append(names, pod.Name)
		}
	}
	sort.Strings(names)
	return names
}

// matchingPDBNames returns the names of PodDisruptionBudgets in namespace
// whose selector matches the same pods as selector -- evidence only (see
// DRAIN001's doc comment for why PDB presence doesn't change severity).
func matchingPDBNames(snap *k8s.Snapshot, namespace string, selector *metav1.LabelSelector) []string {
	if selector == nil {
		return nil
	}
	workloadSel, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil
	}
	var names []string
	for _, pdb := range snap.PodDisruptionBudgets {
		if pdb.Namespace != namespace || pdb.Spec.Selector == nil {
			continue
		}
		pdbSel, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
		if err != nil {
			continue
		}
		for _, pod := range snap.Pods {
			if pod.Namespace != namespace || !isLiveWorkloadPod(pod) {
				continue
			}
			podLabels := labels.Set(pod.Labels)
			if workloadSel.Matches(podLabels) && pdbSel.Matches(podLabels) {
				names = append(names, pdb.Name)
				break
			}
		}
	}
	sort.Strings(names)
	return names
}

type drain001Params struct {
	Kind                             string
	Namespace, Name, UID             string
	DesiredReplicas                  int32
	ReadyReplicas, AvailableReplicas int32
	RolloutStrategy                  string
	PDBNames, AffectedPods           []string
}

func drain001Finding(p drain001Params, targetVersion string) findings.Finding {
	pdbNote := "no PodDisruptionBudget protects this workload"
	if len(p.PDBNames) > 0 {
		pdbNote = fmt.Sprintf("protected by PodDisruptionBudget(s) %s, which governs whether eviction is currently permitted but does not add replacement capacity", strings.Join(p.PDBNames, ", "))
	}
	msg := fmt.Sprintf(
		"%s %s/%s runs a single replica (desired: %d, ready: %d, available: %d) — when its pod is evicted (node drain, node upgrade, or any voluntary disruption), this workload has zero available replicas until a replacement schedules and becomes Ready elsewhere; %s",
		p.Kind, p.Namespace, p.Name, p.DesiredReplicas, p.ReadyReplicas, p.AvailableReplicas, pdbNote)

	remediation := "Increase replicas to create real eviction headroom (a healthy PDB alone doesn't prevent the capacity gap), or explicitly accept single-replica downtime for this workload and document it. " +
		"If this workload can't run multiple replicas (e.g. a singleton controller with leader election), consider a PodDisruptionBudget with minAvailable: 0 combined with a documented manual coordination process for drains."

	ref := findings.LiveResource(p.Kind, findings.ScopeNamespaced, p.Namespace, p.Name, p.UID)
	evidence := []string{
		fmt.Sprintf("desired replicas: %d", p.DesiredReplicas),
		fmt.Sprintf("ready replicas: %d", p.ReadyReplicas),
		fmt.Sprintf("available replicas: %d", p.AvailableReplicas),
		fmt.Sprintf("rollout strategy: %s", p.RolloutStrategy),
	}
	if len(p.PDBNames) > 0 {
		evidence = append(evidence, fmt.Sprintf("PodDisruptionBudget(s): %s", strings.Join(p.PDBNames, ", ")))
	} else {
		evidence = append(evidence, "PodDisruptionBudget(s): none")
	}
	if len(p.AffectedPods) > 0 {
		evidence = append(evidence, fmt.Sprintf("affected pod(s): %s", strings.Join(p.AffectedPods, ", ")))
	}

	return findings.Finding{
		RuleID:      "DRAIN-001",
		Severity:    findings.SeverityWarning,
		Confidence:  findings.TierStaticCertain,
		Message:     msg,
		Resources:   []findings.ResourceReference{ref},
		Evidence:    evidence,
		Remediation: remediation,
		Fingerprint: findings.FingerprintV2("DRAIN-001", targetVersion, "", ref),
	}
}

func drain001ManifestFinding(obj manifest.WorkloadObject, targetVersion string) findings.Finding {
	resourceLabel := obj.Name
	if obj.Namespace != "" {
		resourceLabel = obj.Namespace + "/" + obj.Name
	}
	msg := fmt.Sprintf(
		"%s %q (%s) declares a single replica — when its pod is evicted (node drain, node upgrade, or any voluntary disruption), this workload has zero available replicas until a replacement schedules and becomes Ready elsewhere. "+
			"This is a manifest-plane finding: live replica status and PodDisruptionBudget relationship aren't available for a not-yet-applied manifest.",
		obj.Kind, resourceLabel, obj.SourcePath)

	remediation := "Increase replicas to create real eviction headroom before this manifest is applied, or explicitly accept single-replica downtime for this workload and document it."

	ref := findings.ManifestResource(obj.Kind, findings.ScopeNamespaced, obj.Namespace, obj.Name, obj.SourcePath)
	return findings.Finding{
		RuleID:     "DRAIN-001",
		Severity:   findings.SeverityWarning,
		Confidence: findings.TierStaticCertain,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		Evidence: []string{
			"desired replicas: 1 (spec.replicas explicit or unset default)",
			fmt.Sprintf("source: %s", obj.SourcePath),
		},
		Remediation: remediation,
		Fingerprint: findings.FingerprintV2("DRAIN-001", targetVersion, "", ref),
	}
}
