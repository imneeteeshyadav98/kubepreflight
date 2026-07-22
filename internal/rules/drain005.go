package rules

import (
	"fmt"
	"sort"

	appsv1 "k8s.io/api/apps/v1"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

// DRAIN005 flags StatefulSets and DaemonSets that currently don't have
// all their desired pods Ready -- proceeding with a node drain while a
// StatefulSet or DaemonSet is already short of capacity compounds the
// risk on top of whatever the drain itself causes.
//
// Deliberately does not duplicate DRAIN-001 (singleton replica count) or
// DRAIN-002 (node-local storage) -- this rule reads only Ready/Available
// counts and rollout metadata, a currently-observed fact rather than a
// structural one. A StatefulSet can trigger both DRAIN-001 (replicas: 1)
// and DRAIN-005 (that one replica isn't Ready) simultaneously; that's
// expected, not a duplicate -- they answer different questions ("is
// there any headroom at all" vs "is this workload healthy right now").
//
// Severity defaults to Warning: a StatefulSet/DaemonSet with some but not
// all replicas Ready is very often just a normal in-progress rollout, not
// a stuck one -- this tool has no rollout-duration/event-history signal
// to distinguish "5 seconds into a routine update" from "stuck for an
// hour," so it doesn't try. Escalates to Blocker only for the one
// condition that's true regardless of how long it's been that way: zero
// Ready pods out of a nonzero desired count -- the workload is completely
// down right now, a proven fact rather than an inferred one. Critical
// infrastructure DaemonSets/StatefulSets (same well-known name list
// node003.go already tracks) get CriticalInfra escalation to at least P2
// via the existing mechanism, regardless of severity.
type DRAIN005 struct{}

func (DRAIN005) ID() string { return "DRAIN-005" }

func (DRAIN005) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	if sc == nil || sc.K8s == nil {
		return nil, nil
	}
	snap := sc.K8s
	var out []findings.Finding

	for _, sts := range snap.StatefulSets {
		if sts.DeletionTimestamp != nil {
			continue
		}
		if f, ok := drain005StatefulSetFinding(sts, targetVersion, scanUpgradeContext(sc)); ok {
			out = append(out, f)
		}
	}
	for _, ds := range snap.DaemonSets {
		if ds.DeletionTimestamp != nil {
			continue
		}
		if f, ok := drain005DaemonSetFinding(ds, targetVersion, scanUpgradeContext(sc)); ok {
			out = append(out, f)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Fingerprint < out[j].Fingerprint
	})
	return out, nil
}

func drain005StatefulSetFinding(sts appsv1.StatefulSet, targetVersion string, upgradeContext findings.UpgradeContext) (findings.Finding, bool) {
	replicas := int32(1)
	if sts.Spec.Replicas != nil {
		replicas = *sts.Spec.Replicas
	}
	if replicas == 0 || sts.Status.ReadyReplicas >= replicas {
		return findings.Finding{}, false
	}

	critical := isCriticalInfraName(sts.Name)
	zeroReady := sts.Status.ReadyReplicas == 0
	severity, gate := currentHealthGate(upgradeContext, critical, zeroReady)

	strategy := string(sts.Spec.UpdateStrategy.Type)
	if strategy == "" {
		strategy = "RollingUpdate"
	}
	updateInProgress := sts.Status.CurrentRevision != "" && sts.Status.UpdateRevision != "" && sts.Status.CurrentRevision != sts.Status.UpdateRevision
	partitionNote := ""
	if sts.Spec.UpdateStrategy.RollingUpdate != nil && sts.Spec.UpdateStrategy.RollingUpdate.Partition != nil && *sts.Spec.UpdateStrategy.RollingUpdate.Partition > 0 {
		partitionNote = fmt.Sprintf(" (partition: %d — ordinals below this are intentionally held back)", *sts.Spec.UpdateStrategy.RollingUpdate.Partition)
	}

	msg := fmt.Sprintf(
		"StatefulSet %s/%s: %d/%d replicas Ready — this workload has less capacity than desired right now, before any drain adds further disruption",
		sts.Namespace, sts.Name, sts.Status.ReadyReplicas, replicas)
	if zeroReady {
		msg += " (zero Ready replicas: this workload is completely down)"
	}

	remediation := "Investigate why this StatefulSet isn't fully Ready before starting or continuing a node drain -- check pod events, image pulls, and readiness probes. " +
		"Resolve the underlying issue rather than proceeding with maintenance on top of an already-degraded workload."

	evidence := []string{
		fmt.Sprintf("desired replicas: %d", replicas),
		fmt.Sprintf("ready replicas: %d", sts.Status.ReadyReplicas),
		fmt.Sprintf("current replicas: %d", sts.Status.CurrentReplicas),
		fmt.Sprintf("updated replicas: %d", sts.Status.UpdatedReplicas),
		fmt.Sprintf("podManagementPolicy: %s", podManagementPolicyOrDefault(sts.Spec.PodManagementPolicy)),
		fmt.Sprintf("updateStrategy: %s%s", strategy, partitionNote),
		fmt.Sprintf("update in progress: %t", updateInProgress),
	}

	ref := findings.LiveResource("StatefulSet", findings.ScopeNamespaced, sts.Namespace, sts.Name, string(sts.UID))
	return findings.Finding{
		RuleID:        "DRAIN-005",
		Severity:      severity,
		Confidence:    findings.TierObserved,
		Message:       msg,
		Resources:     []findings.ResourceReference{ref},
		Evidence:      evidence,
		Remediation:   remediation,
		CriticalInfra: critical,
		ImpactScopes: []findings.ImpactScope{
			findings.ImpactScopeCurrentHealth,
			findings.ImpactScopeWorkerRollout,
			findings.ImpactScopeNodeDrain,
			findings.ImpactScopeWorkloadRestart,
		},
		UpgradeGate: gate,
		Fingerprint: findings.FingerprintV2("DRAIN-005", targetVersion, "statefulset", ref),
	}, true
}

func drain005DaemonSetFinding(ds appsv1.DaemonSet, targetVersion string, upgradeContext findings.UpgradeContext) (findings.Finding, bool) {
	if ds.Status.DesiredNumberScheduled == 0 || ds.Status.NumberUnavailable == 0 {
		return findings.Finding{}, false
	}

	critical := isCriticalInfraName(ds.Name)
	zeroReady := ds.Status.NumberReady == 0
	severity, gate := currentHealthGate(upgradeContext, critical, zeroReady)

	strategy := string(ds.Spec.UpdateStrategy.Type)
	if strategy == "" {
		strategy = "RollingUpdate"
	}
	maxUnavailable := "1 (default)"
	if ds.Spec.UpdateStrategy.RollingUpdate != nil && ds.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable != nil {
		maxUnavailable = ds.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable.String()
	}

	msg := fmt.Sprintf(
		"DaemonSet %s/%s: %d/%d nodes have a Ready pod (%d unavailable) — this node agent has less coverage than desired right now, before any drain adds further disruption",
		ds.Namespace, ds.Name, ds.Status.NumberReady, ds.Status.DesiredNumberScheduled, ds.Status.NumberUnavailable)
	if zeroReady {
		msg += " (zero Ready pods: this node agent is completely down cluster-wide)"
	}
	if critical {
		msg += " -- this is a cluster-critical node agent"
	}

	remediation := "Investigate why this DaemonSet isn't fully Ready before starting or continuing a node drain -- check pod events, image pulls, readiness probes, and whether affected nodes have taints this DaemonSet doesn't tolerate. " +
		"A degraded critical node agent (CNI, kube-proxy, CSI driver) can affect networking/storage on every node it's missing from, not just the pods it directly serves."

	ref := findings.LiveResource("DaemonSet", findings.ScopeNamespaced, ds.Namespace, ds.Name, string(ds.UID))
	return findings.Finding{
		RuleID:     "DRAIN-005",
		Severity:   severity,
		Confidence: findings.TierObserved,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		Evidence: []string{
			fmt.Sprintf("desired nodes: %d", ds.Status.DesiredNumberScheduled),
			fmt.Sprintf("ready: %d", ds.Status.NumberReady),
			fmt.Sprintf("available: %d", ds.Status.NumberAvailable),
			fmt.Sprintf("unavailable: %d", ds.Status.NumberUnavailable),
			fmt.Sprintf("updateStrategy: %s (maxUnavailable: %s)", strategy, maxUnavailable),
		},
		Remediation:   remediation,
		CriticalInfra: critical,
		ImpactScopes: []findings.ImpactScope{
			findings.ImpactScopeCurrentHealth,
			findings.ImpactScopeWorkerRollout,
			findings.ImpactScopeNodeDrain,
			findings.ImpactScopeWorkloadRestart,
		},
		UpgradeGate: gate,
		Fingerprint: findings.FingerprintV2("DRAIN-005", targetVersion, "daemonset", ref),
	}, true
}

func podManagementPolicyOrDefault(p appsv1.PodManagementPolicyType) string {
	if p == "" {
		return "OrderedReady (default)"
	}
	return string(p)
}

// isCriticalInfraName reuses node003.go's well-known critical-component
// name list rather than maintaining a second, driftable copy -- same
// deliberate exact-match-only design (extend with a test per name, not a
// fuzzy heuristic).
func isCriticalInfraName(name string) bool {
	_, ok := node003CriticalComponentNames[name]
	return ok
}
