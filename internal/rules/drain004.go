package rules

import (
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"kubepreflight/internal/collectors/k8s"
	"kubepreflight/internal/findings"
)

// DRAIN004 estimates whether the cluster's remaining nodes have enough
// spare CPU/memory (allocatable minus already-requested) to absorb one
// node's non-DaemonSet pods if that node is removed -- the question a
// node-group rolling upgrade or manual drain actually poses.
//
// This is an aggregate estimate, not a scheduler simulation (explicit
// non-goal): it sums CPU and memory separately across all remaining
// nodes and compares against total displaced demand. That makes an
// INSUFFICIENT result trustworthy (aggregate demand exceeding aggregate
// supply is impossible to resolve regardless of bin-packing), but a
// SUFFICIENT result is not a full guarantee -- individual pods could
// still fail to find one node with enough of both resources
// simultaneously even when the totals look fine. Every finding's
// evidence and remediation say this explicitly.
//
// Severity is capped at Warning, never Blocker, deliberately: this repo
// has no reliable live signal that a cluster autoscaler (Cluster
// Autoscaler, Karpenter, or a differently-named/managed equivalent) is
// absent, so a "no escape known" claim can never be proven -- only "no
// escape observed." A capacity estimate that could fail CI on an
// unprovable claim is exactly the false-positive risk this rule exists
// to avoid, not create.
//
// Uses only resource REQUESTS, never actual usage (a non-goal per the
// design doc: "resources absent hone par capacity invent na karo"), and
// pods with no requests at all are excluded from the demand sum rather
// than assumed zero-cost -- their count is surfaced as an explicit
// coverage caveat instead, since assuming zero would understate risk in
// a way that's invisible to the reader.
type DRAIN004 struct{}

func (DRAIN004) ID() string { return "DRAIN-004" }

func (DRAIN004) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	if sc == nil || sc.K8s == nil {
		return nil, nil
	}
	snap := sc.K8s
	if len(snap.Nodes) < 2 {
		// Nothing to redistribute onto -- a single-node cluster has no
		// "remaining nodes" concept, and an empty node list means nothing
		// to evaluate.
		return nil, nil
	}

	usage := computeNodeUsage(snap)
	pendingDemand, pendingMissingRequests := pendingPodDemand(snap)

	var out []findings.Finding
	for _, node := range snap.Nodes {
		if !nodeIsReady(node) || node.Spec.Unschedulable {
			continue
		}
		displaced, missingOnNode := displacedDemand(snap, node.Name)
		if displaced.cpuMilli == 0 && displaced.memBytes == 0 && missingOnNode == 0 {
			continue
		}
		totalDemandCPU := displaced.cpuMilli + pendingDemand.cpuMilli
		totalDemandMem := displaced.memBytes + pendingDemand.memBytes

		spareCPU, spareMem := int64(0), int64(0)
		for _, other := range snap.Nodes {
			if other.Name == node.Name || !nodeIsReady(other) || other.Spec.Unschedulable || hasControlPlaneTaint(other) {
				continue
			}
			alloc := usage[other.Name].allocatable
			used := usage[other.Name].requested
			if alloc.cpuMilli > used.cpuMilli {
				spareCPU += alloc.cpuMilli - used.cpuMilli
			}
			if alloc.memBytes > used.memBytes {
				spareMem += alloc.memBytes - used.memBytes
			}
		}

		shortageCPU := totalDemandCPU > spareCPU
		shortageMem := totalDemandMem > spareMem
		if !shortageCPU && !shortageMem {
			continue
		}
		out = append(out, drain004Finding(node, displaced, pendingDemand, spareCPU, spareMem, shortageCPU, shortageMem,
			missingOnNode+pendingMissingRequests, targetVersion))
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Fingerprint < out[j].Fingerprint
	})
	return out, nil
}

type resourceTotal struct {
	cpuMilli int64
	memBytes int64
}

type nodeUsage struct {
	allocatable resourceTotal
	requested   resourceTotal
}

// computeNodeUsage sums allocatable capacity and already-requested
// resources (from every live, scheduled pod, DaemonSet-owned or not --
// this is "what's already consumed on this node today," unlike
// displacedDemand below, which deliberately excludes DaemonSet pods).
func computeNodeUsage(snap *k8s.Snapshot) map[string]nodeUsage {
	usage := map[string]nodeUsage{}
	for _, node := range snap.Nodes {
		usage[node.Name] = nodeUsage{allocatable: resourceTotal{
			cpuMilli: node.Status.Allocatable.Cpu().MilliValue(),
			memBytes: node.Status.Allocatable.Memory().Value(),
		}}
	}
	for _, pod := range snap.Pods {
		if pod.Spec.NodeName == "" || !isLiveWorkloadPod(pod) {
			continue
		}
		u, ok := usage[pod.Spec.NodeName]
		if !ok {
			continue
		}
		reqCPU, reqMem, _ := podRequests(pod)
		u.requested.cpuMilli += reqCPU
		u.requested.memBytes += reqMem
		usage[pod.Spec.NodeName] = u
	}
	return usage
}

// displacedDemand sums the requests of nodeName's own live pods that
// would need to reschedule elsewhere if nodeName were removed --
// DaemonSet-owned pods are excluded, since a DaemonSet pod isn't evicted
// and rescheduled onto a remaining node; it simply stops existing with
// its node and a fresh instance only starts on a genuinely new node, if
// one appears. missingRequests counts pods excluded from the sum because
// at least one container had no CPU or memory request at all.
func displacedDemand(snap *k8s.Snapshot, nodeName string) (total resourceTotal, missingRequests int) {
	for _, pod := range snap.Pods {
		if pod.Spec.NodeName != nodeName || !isLiveWorkloadPod(pod) {
			continue
		}
		if podOwnedByDaemonSet(pod) {
			continue
		}
		cpu, mem, complete := podRequests(pod)
		if !complete {
			missingRequests++
			continue
		}
		total.cpuMilli += cpu
		total.memBytes += mem
	}
	return total, missingRequests
}

// pendingPodDemand sums requests of pods that exist but aren't scheduled
// to any node yet (Pending, NodeName unset) -- they compete for the same
// spare capacity a displaced pod would need, so they're added to total
// demand rather than ignored.
func pendingPodDemand(snap *k8s.Snapshot) (total resourceTotal, missingRequests int) {
	for _, pod := range snap.Pods {
		if pod.Spec.NodeName != "" || pod.Status.Phase != corev1.PodPending || pod.DeletionTimestamp != nil {
			continue
		}
		cpu, mem, complete := podRequests(pod)
		if !complete {
			missingRequests++
			continue
		}
		total.cpuMilli += cpu
		total.memBytes += mem
	}
	return total, missingRequests
}

func podOwnedByDaemonSet(pod corev1.Pod) bool {
	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "DaemonSet" {
			return true
		}
	}
	return false
}

// hasControlPlaneTaint reports whether node carries the standard
// NoSchedule/NoExecute control-plane taint (current
// node-role.kubernetes.io/control-plane or legacy
// node-role.kubernetes.io/master, same well-known keys node003.go
// already tracks) -- ordinary workload pods essentially never tolerate
// this, so counting a control-plane node's allocatable capacity as
// "spare" for displaced workload pods would silently overstate available
// capacity. This is a deliberate, narrow exception: DRAIN-004 doesn't
// otherwise model custom taints/tolerations for capacity purposes (that
// would reintroduce the scheduler-simulation complexity this rule is
// explicitly not attempting) -- only the single most universal and
// clear-cut case, present on every cluster.
func hasControlPlaneTaint(node corev1.Node) bool {
	for _, t := range node.Spec.Taints {
		if (t.Key == deprecatedMasterNodeLabel || t.Key == replacementControlPlaneLabel) &&
			(t.Effect == corev1.TaintEffectNoSchedule || t.Effect == corev1.TaintEffectNoExecute) {
			return true
		}
	}
	return false
}

// podRequests sums a pod's container CPU/memory requests. complete is
// false if any container has no CPU or memory request set at all --
// callers must not silently treat that as zero-cost.
func podRequests(pod corev1.Pod) (cpuMilli, memBytes int64, complete bool) {
	complete = true
	for _, c := range pod.Spec.Containers {
		cpu, hasCPU := c.Resources.Requests[corev1.ResourceCPU]
		mem, hasMem := c.Resources.Requests[corev1.ResourceMemory]
		if !hasCPU || !hasMem {
			complete = false
			continue
		}
		cpuMilli += cpu.MilliValue()
		memBytes += mem.Value()
	}
	return cpuMilli, memBytes, complete
}

func drain004Finding(node corev1.Node, displaced, pending resourceTotal, spareCPU, spareMem int64, shortageCPU, shortageMem bool, missingRequests int, targetVersion string) findings.Finding {
	var shortageParts []string
	if shortageCPU {
		shortageParts = append(shortageParts, fmt.Sprintf("CPU: need %s, remaining nodes have %s spare", formatMilliCPU(displaced.cpuMilli+pending.cpuMilli), formatMilliCPU(spareCPU)))
	}
	if shortageMem {
		shortageParts = append(shortageParts, fmt.Sprintf("memory: need %s, remaining nodes have %s spare", formatBytes(displaced.memBytes+pending.memBytes), formatBytes(spareMem)))
	}

	coverageNote := ""
	if missingRequests > 0 {
		coverageNote = fmt.Sprintf(" (%d pod(s) with incomplete resource requests were excluded from this estimate, so actual demand may be higher)", missingRequests)
	}

	msg := fmt.Sprintf(
		"Node %q: if this node is removed, its non-DaemonSet pods' resource requests exceed the estimated spare capacity of the remaining nodes — %s%s. "+
			"This is an aggregate estimate (not a scheduler simulation): individual pods could still fail to find a single node with enough of both resources even when totals look adequate elsewhere",
		node.Name, joinWithAnd(shortageParts), coverageNote)

	remediation := "Add node capacity (or increase managed node group size) before draining this node, or right-size workload requests if they're overstated. " +
		"This estimate uses resource requests only, not actual usage, and assumes no cluster autoscaler adds capacity -- if autoscaling is configured, this may be a false alarm; verify against your autoscaler's logs/events."

	evidence := []string{
		fmt.Sprintf("displaced demand (this node's non-DaemonSet pods): CPU %s, memory %s", formatMilliCPU(displaced.cpuMilli), formatBytes(displaced.memBytes)),
		fmt.Sprintf("pending/unscheduled demand: CPU %s, memory %s", formatMilliCPU(pending.cpuMilli), formatBytes(pending.memBytes)),
		fmt.Sprintf("estimated spare capacity on remaining nodes: CPU %s, memory %s", formatMilliCPU(spareCPU), formatBytes(spareMem)),
	}
	if missingRequests > 0 {
		evidence = append(evidence, fmt.Sprintf("pods excluded from this estimate due to incomplete resource requests: %d", missingRequests))
	}

	ref := findings.LiveResource("Node", findings.ScopeCluster, "", node.Name, string(node.UID))
	return findings.Finding{
		RuleID:      "DRAIN-004",
		Severity:    findings.SeverityWarning,
		Confidence:  findings.TierInferred,
		Message:     msg,
		Resources:   []findings.ResourceReference{ref},
		Evidence:    evidence,
		Remediation: remediation,
		Fingerprint: findings.FingerprintV2("DRAIN-004", targetVersion, "", ref),
	}
}

func formatMilliCPU(m int64) string {
	return resource.NewMilliQuantity(m, resource.DecimalSI).String()
}

func formatBytes(b int64) string {
	return resource.NewQuantity(b, resource.BinarySI).String()
}

func joinWithAnd(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	default:
		return parts[0] + "; " + parts[1]
	}
}
