package rules

import (
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/k8s"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

// drainHostnameTopologyKey is the well-known node label used for
// "one pod per node" anti-affinity/topology-spread patterns.
const drainHostnameTopologyKey = "kubernetes.io/hostname"

// DRAIN003 flags Deployment/StatefulSet pod templates with a hard
// scheduling constraint that a replacement pod may not be able to satisfy
// once the current node is gone: node affinity/selector scarcity,
// required anti-affinity with no spare node, a topology-spread
// DoNotSchedule constraint collapsed to one domain, or a hostPort with no
// free alternate node.
//
// Every check here proves unschedulability against the CURRENT node
// inventory -- it never treats "this pod has affinity/anti-affinity/a
// topology spread constraint" alone as risky (most workloads that use
// these features schedule just fine). The shared foundation is
// qualifyingNodes: which live nodes actually satisfy this pod template's
// nodeSelector/nodeAffinity and whose taints its tolerations cover -- the
// same node-eligibility question the real scheduler asks, evaluated once
// and reused by all four sub-checks below.
type DRAIN003 struct{}

func (DRAIN003) ID() string { return "DRAIN-003" }

func (DRAIN003) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	if sc == nil || sc.K8s == nil {
		return nil, nil
	}
	snap := sc.K8s
	var out []findings.Finding

	for _, d := range snap.Deployments {
		if d.DeletionTimestamp != nil {
			continue
		}
		replicas := int32(1)
		if d.Spec.Replicas != nil {
			replicas = *d.Spec.Replicas
		}
		out = append(out, drain003Evaluate(snap, "Deployment", d.ObjectMeta, d.Spec.Template.Spec, replicas, targetVersion)...)
	}
	for _, sts := range snap.StatefulSets {
		if sts.DeletionTimestamp != nil {
			continue
		}
		replicas := int32(1)
		if sts.Spec.Replicas != nil {
			replicas = *sts.Spec.Replicas
		}
		out = append(out, drain003Evaluate(snap, "StatefulSet", sts.ObjectMeta, sts.Spec.Template.Spec, replicas, targetVersion)...)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Fingerprint < out[j].Fingerprint
	})
	return out, nil
}

func drain003Evaluate(snap *k8s.Snapshot, kind string, meta metav1.ObjectMeta, podSpec corev1.PodSpec, replicas int32, targetVersion string) []findings.Finding {
	var out []findings.Finding

	if f, ok := drain003ScarcityFinding(snap, kind, meta, podSpec, targetVersion); ok {
		out = append(out, f)
	}
	if f, ok := drain003AntiAffinityFinding(snap, kind, meta, podSpec, replicas, targetVersion); ok {
		out = append(out, f)
	}
	if f, ok := drain003TopologySpreadFinding(snap, kind, meta, podSpec, targetVersion); ok {
		out = append(out, f)
	}
	if f, ok := drain003HostPortFinding(snap, kind, meta, podSpec, targetVersion); ok {
		out = append(out, f)
	}
	return out
}

// drain003ScarcityFinding fires when a pod template's nodeSelector/required
// nodeAffinity, combined with what its tolerations actually cover, is
// satisfied by at most one live node today -- draining that node leaves no
// other currently-known target.
func drain003ScarcityFinding(snap *k8s.Snapshot, kind string, meta metav1.ObjectMeta, podSpec corev1.PodSpec, targetVersion string) (findings.Finding, bool) {
	if len(podSpec.NodeSelector) == 0 && !hasRequiredNodeAffinity(podSpec) {
		return findings.Finding{}, false
	}
	qualifying := qualifyingNodes(snap, podSpec)
	if len(qualifying) > 1 {
		return findings.Finding{}, false
	}
	names := nodeNames(qualifying)
	msg := fmt.Sprintf(
		"%s %s/%s has a nodeSelector/required nodeAffinity satisfied by only %d node(s) in this cluster today (%s) — if that node is drained, no other currently-known node can host a replacement pod",
		kind, meta.Namespace, meta.Name, len(qualifying), nodeListOrNone(names))
	return drain003Finding(kind, meta, "scarcity", targetVersion, msg,
		[]string{fmt.Sprintf("qualifying node(s): %s", nodeListOrNone(names))},
		"Label additional nodes to match this workload's nodeSelector/nodeAffinity (and taint them consistently if tolerations are also required), or relax the constraint if it's broader than actually needed.",
	), true
}

// drain003AntiAffinityFinding fires when a pod template requires
// hostname-topology anti-affinity (the common "one replica per node"
// pattern) and the qualifying node count leaves no spare node beyond the
// desired replica count.
func drain003AntiAffinityFinding(snap *k8s.Snapshot, kind string, meta metav1.ObjectMeta, podSpec corev1.PodSpec, replicas int32, targetVersion string) (findings.Finding, bool) {
	if !hasHostnameAntiAffinity(podSpec) {
		return findings.Finding{}, false
	}
	qualifying := qualifyingNodes(snap, podSpec)
	if int32(len(qualifying)) > replicas {
		return findings.Finding{}, false
	}
	names := nodeNames(qualifying)
	msg := fmt.Sprintf(
		"%s %s/%s requires one-pod-per-node anti-affinity with %d desired replica(s), but only %d node(s) qualify to run it (%s) — there's no spare node for a replacement pod to land on without violating anti-affinity during a drain",
		kind, meta.Namespace, meta.Name, replicas, len(qualifying), nodeListOrNone(names))
	return drain003Finding(kind, meta, "anti-affinity", targetVersion, msg,
		[]string{fmt.Sprintf("desired replicas: %d", replicas), fmt.Sprintf("qualifying node(s): %s", nodeListOrNone(names))},
		"Add spare qualifying nodes (at least one more than desired replicas), or relax the anti-affinity rule if strict one-per-node placement isn't actually required.",
	), true
}

// drain003TopologySpreadFinding fires when a DoNotSchedule topology spread
// constraint's qualifying nodes all collapse into a single topology
// domain -- there's no second domain to absorb a replacement pod without
// breaching the constraint.
func drain003TopologySpreadFinding(snap *k8s.Snapshot, kind string, meta metav1.ObjectMeta, podSpec corev1.PodSpec, targetVersion string) (findings.Finding, bool) {
	tsc, ok := doNotScheduleConstraint(podSpec)
	if !ok {
		return findings.Finding{}, false
	}
	qualifying := qualifyingNodes(snap, podSpec)
	domains := map[string]struct{}{}
	for _, n := range qualifying {
		if v, ok := n.Labels[tsc.TopologyKey]; ok {
			domains[v] = struct{}{}
		}
	}
	if len(domains) > 1 {
		return findings.Finding{}, false
	}
	msg := fmt.Sprintf(
		"%s %s/%s has a topologySpreadConstraint (topologyKey: %s, whenUnsatisfiable: DoNotSchedule) whose qualifying nodes all fall into %d topology domain — there's no second domain for a replacement pod to land in without breaching maxSkew during a drain",
		kind, meta.Namespace, meta.Name, tsc.TopologyKey, len(domains))
	return drain003Finding(kind, meta, "topology-spread", targetVersion, msg,
		[]string{fmt.Sprintf("topologyKey: %s", tsc.TopologyKey), fmt.Sprintf("distinct domains among qualifying nodes: %d", len(domains))},
		"Add qualifying nodes in at least one more topology domain, or relax whenUnsatisfiable to ScheduleAnyway if a temporarily imbalanced placement is acceptable.",
	), true
}

// drain003HostPortFinding fires when a pod template reserves a hostPort
// and every other live node already has a different pod bound to that
// same port -- an evicted pod has nowhere else to land.
func drain003HostPortFinding(snap *k8s.Snapshot, kind string, meta metav1.ObjectMeta, podSpec corev1.PodSpec, targetVersion string) (findings.Finding, bool) {
	ports := hostPorts(podSpec)
	if len(ports) == 0 {
		return findings.Finding{}, false
	}
	currentNode := drain002RunningNodeName(snap, meta.Namespace, meta.Name, kind)
	occupied := hostPortsByNode(snap)

	var conflicts []string
	for _, port := range ports {
		freeElsewhere := false
		for _, node := range snap.Nodes {
			if node.Name == currentNode {
				continue
			}
			if !occupied[node.Name][port] {
				freeElsewhere = true
				break
			}
		}
		if !freeElsewhere && len(snap.Nodes) > 0 {
			conflicts = append(conflicts, fmt.Sprintf("%d", port))
		}
	}
	if len(conflicts) == 0 {
		return findings.Finding{}, false
	}
	msg := fmt.Sprintf(
		"%s %s/%s reserves hostPort(s) %s, and every other live node already has a different pod bound to the same port — if this pod is evicted, it has nowhere else to schedule",
		kind, meta.Namespace, meta.Name, strings.Join(conflicts, ", "))
	return drain003Finding(kind, meta, "hostport:"+strings.Join(conflicts, ","), targetVersion, msg,
		[]string{fmt.Sprintf("hostPort(s) with no free alternate node: %s", strings.Join(conflicts, ", ")), fmt.Sprintf("currently running on node: %s", drain002NodeLabel(currentNode))},
		"Avoid hostPort if possible (use a Service/NodePort instead), or ensure enough nodes have this port free to absorb an evicted pod.",
	), true
}

func hasRequiredNodeAffinity(podSpec corev1.PodSpec) bool {
	return podSpec.Affinity != nil && podSpec.Affinity.NodeAffinity != nil &&
		podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil &&
		len(podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms) > 0
}

// hasHostnameAntiAffinity reports whether podSpec has a required pod
// anti-affinity term keyed on the hostname topology -- the "one replica
// per node" pattern. Other topology keys (zone, region) are out of scope:
// reasoning about spare-domain capacity for arbitrary topology keys is
// exactly what the topology-spread check already covers for the
// DoNotSchedule case, and required anti-affinity on a non-hostname key
// without a matching spread constraint is rare enough to leave for a
// later PR rather than risk a false positive from an under-tested path.
func hasHostnameAntiAffinity(podSpec corev1.PodSpec) bool {
	if podSpec.Affinity == nil || podSpec.Affinity.PodAntiAffinity == nil {
		return false
	}
	for _, term := range podSpec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
		if term.TopologyKey == drainHostnameTopologyKey {
			return true
		}
	}
	return false
}

func doNotScheduleConstraint(podSpec corev1.PodSpec) (corev1.TopologySpreadConstraint, bool) {
	for _, tsc := range podSpec.TopologySpreadConstraints {
		if tsc.WhenUnsatisfiable == corev1.DoNotSchedule {
			return tsc, true
		}
	}
	return corev1.TopologySpreadConstraint{}, false
}

func hostPorts(podSpec corev1.PodSpec) []int32 {
	var ports []int32
	for _, c := range podSpec.Containers {
		for _, p := range c.Ports {
			if p.HostPort != 0 {
				ports = append(ports, p.HostPort)
			}
		}
	}
	sort.Slice(ports, func(i, j int) bool { return ports[i] < ports[j] })
	return ports
}

// hostPortsByNode maps every live node currently hosting a pod that
// reserves a hostPort to the set of ports reserved there, across ALL pods
// in the cluster (not just the workload under evaluation) -- a hostPort
// conflict is with whatever else is scheduled, not just sibling replicas.
func hostPortsByNode(snap *k8s.Snapshot) map[string]map[int32]bool {
	byNode := map[string]map[int32]bool{}
	for _, pod := range snap.Pods {
		if pod.Spec.NodeName == "" || !isLiveWorkloadPod(pod) {
			continue
		}
		for _, c := range pod.Spec.Containers {
			for _, p := range c.Ports {
				if p.HostPort == 0 {
					continue
				}
				if byNode[pod.Spec.NodeName] == nil {
					byNode[pod.Spec.NodeName] = map[int32]bool{}
				}
				byNode[pod.Spec.NodeName][p.HostPort] = true
			}
		}
	}
	return byNode
}

// qualifyingNodes returns the live nodes that satisfy podSpec's
// nodeSelector AND required nodeAffinity (Kubernetes requires both when
// both are set) AND whose taints podSpec's tolerations actually cover --
// the same node-eligibility question the real scheduler asks. Only
// matchExpressions are evaluated for nodeAffinity terms (not
// matchFields, which in practice is only used for metadata.name and is
// rare); a term with only matchFields is treated as not matching, which
// is conservative (never overstates how many nodes qualify).
func qualifyingNodes(snap *k8s.Snapshot, podSpec corev1.PodSpec) []corev1.Node {
	var out []corev1.Node
	for _, node := range snap.Nodes {
		if !nodeMatchesSelector(node, podSpec.NodeSelector) {
			continue
		}
		if hasRequiredNodeAffinity(podSpec) && !nodeMatchesRequiredAffinity(node, podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution) {
			continue
		}
		if !nodeToleratesTaints(node, podSpec.Tolerations) {
			continue
		}
		out = append(out, node)
	}
	return out
}

func nodeMatchesSelector(node corev1.Node, selector map[string]string) bool {
	for k, v := range selector {
		if node.Labels[k] != v {
			return false
		}
	}
	return true
}

func nodeMatchesRequiredAffinity(node corev1.Node, req *corev1.NodeSelector) bool {
	for _, term := range req.NodeSelectorTerms {
		if nodeMatchesSelectorTerm(node, term) {
			return true
		}
	}
	return false
}

func nodeMatchesSelectorTerm(node corev1.Node, term corev1.NodeSelectorTerm) bool {
	for _, expr := range term.MatchExpressions {
		if !nodeMatchesExpression(node, expr) {
			return false
		}
	}
	return true
}

func nodeMatchesExpression(node corev1.Node, expr corev1.NodeSelectorRequirement) bool {
	value, present := node.Labels[expr.Key]
	switch expr.Operator {
	case corev1.NodeSelectorOpIn:
		return present && containsString(expr.Values, value)
	case corev1.NodeSelectorOpNotIn:
		return !present || !containsString(expr.Values, value)
	case corev1.NodeSelectorOpExists:
		return present
	case corev1.NodeSelectorOpDoesNotExist:
		return !present
	default:
		// Gt/Lt: numeric comparison, rare for node scheduling in practice
		// -- treated as not-matching (conservative) rather than risking an
		// incorrect numeric parse.
		return false
	}
}

func containsString(values []string, v string) bool {
	for _, x := range values {
		if x == v {
			return true
		}
	}
	return false
}

func nodeToleratesTaints(node corev1.Node, tolerations []corev1.Toleration) bool {
	for _, taint := range node.Spec.Taints {
		tolerated := false
		for _, t := range tolerations {
			if t.ToleratesTaint(&taint) {
				tolerated = true
				break
			}
		}
		if !tolerated {
			return false
		}
	}
	return true
}

func nodeNames(nodes []corev1.Node) []string {
	names := make([]string, len(nodes))
	for i, n := range nodes {
		names[i] = n.Name
	}
	sort.Strings(names)
	return names
}

func nodeListOrNone(names []string) string {
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

func drain003Finding(kind string, meta metav1.ObjectMeta, discriminator, targetVersion, message string, evidence []string, remediation string) findings.Finding {
	ref := findings.LiveResource(kind, findings.ScopeNamespaced, meta.Namespace, meta.Name, string(meta.UID))
	return findings.Finding{
		RuleID:      "DRAIN-003",
		Severity:    findings.SeverityWarning,
		Confidence:  findings.TierObserved,
		Message:     message,
		Resources:   []findings.ResourceReference{ref},
		Evidence:    evidence,
		Remediation: remediation,
		Fingerprint: findings.FingerprintV2("DRAIN-003", targetVersion, discriminator, ref),
	}
}
