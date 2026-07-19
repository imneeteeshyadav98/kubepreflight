package rules

import (
	"fmt"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/k8s"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

// drainHostnameLabel is the well-known label PersistentVolume nodeAffinity
// (and node objects themselves) use to pin storage to one exact node.
const drainHostnameLabel = "kubernetes.io/hostname"

// DRAIN002 flags a Deployment or StatefulSet pod template using hostPath
// or a PersistentVolumeClaim bound to a node-pinned PersistentVolume
// (Local volume source, or any PV whose nodeAffinity requires one exact
// kubernetes.io/hostname value): that pod's data exists only on one node,
// so evicting it (node drain, node upgrade) either strands the data or
// requires the replacement pod to land back on the exact same node.
//
// DaemonSets are out of scope: hostPath there is the intended pattern
// (every node gets its own copy; nothing to evacuate). Network-attached
// CSI storage (EBS, EFS) with AZ-level topology nodeAffinity is
// deliberately NOT treated as node-pinned here -- only an exact single
// kubernetes.io/hostname match counts as genuinely stuck, per the
// false-positive guard locked in the v0.8.0 design doc. emptyDir is
// out of scope entirely for this rule (ephemeral-by-design, not a drain
// risk) -- see the design doc for why it isn't even an Info-level finding
// in this PR.
type DRAIN002 struct{}

func (DRAIN002) ID() string { return "DRAIN-002" }

func (DRAIN002) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	if sc == nil || sc.K8s == nil {
		return nil, nil
	}
	snap := sc.K8s
	var out []findings.Finding

	for _, d := range snap.Deployments {
		if d.DeletionTimestamp != nil {
			continue
		}
		if f, ok := drain002Evaluate(snap, "Deployment", d.ObjectMeta, d.Spec.Template.Spec, isSingletonReplicaCount(d.Spec.Replicas), targetVersion); ok {
			out = append(out, f...)
		}
	}
	for _, sts := range snap.StatefulSets {
		if sts.DeletionTimestamp != nil {
			continue
		}
		podSpec := sts.Spec.Template.Spec
		// volumeClaimTemplates never appear in spec.template.spec.volumes --
		// the StatefulSet controller synthesizes a PersistentVolumeClaim
		// volume per (template, ordinal) directly onto each live pod, named
		// "<template>-<statefulset>-<ordinal>". Confirmed against a real
		// kind cluster: a StatefulSet using rancher.io/local-path storage
		// had an empty spec.template.spec.volumes despite every live pod
		// mounting a real, node-pinned PVC -- without this, DRAIN-002 would
		// silently miss the single most common local-storage pattern
		// (StatefulSet + volumeClaimTemplates) entirely.
		podSpec.Volumes = append(append([]corev1.Volume{}, podSpec.Volumes...), statefulSetSyntheticPVCVolumes(sts)...)
		if f, ok := drain002Evaluate(snap, "StatefulSet", sts.ObjectMeta, podSpec, isSingletonReplicaCount(sts.Spec.Replicas), targetVersion); ok {
			out = append(out, f...)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Fingerprint < out[j].Fingerprint
	})
	return out, nil
}

func drain002Evaluate(snap *k8s.Snapshot, kind string, meta metav1.ObjectMeta, podSpec corev1.PodSpec, singleton bool, targetVersion string) ([]findings.Finding, bool) {
	hostPaths := drain002HostPathVolumes(podSpec)
	localPVs := drain002LocalPVVolumes(snap, meta.Namespace, podSpec)
	if len(hostPaths) == 0 && len(localPVs) == 0 {
		return nil, false
	}

	var out []findings.Finding
	if len(hostPaths) > 0 {
		nodeName := drain002RunningNodeName(snap, meta.Namespace, meta.Name, kind)
		severity, note := drain002Severity(snap, singleton, nodeName)
		out = append(out, drain002Finding(kind, meta, "hostpath", severity, targetVersion,
			fmt.Sprintf("%s %s/%s uses hostPath volume(s) (%s) — this pod's data exists only on whichever node it currently runs on; evicting it (node drain, node upgrade) either strands that data or requires the replacement pod to land back on the exact same node%s",
				kind, meta.Namespace, meta.Name, strings.Join(hostPaths, ", "), note),
			[]string{
				fmt.Sprintf("hostPath volume(s): %s", strings.Join(hostPaths, ", ")),
				fmt.Sprintf("currently running on node: %s", drain002NodeLabel(nodeName)),
			},
			"Migrate to a networked/replicated storage class (or a topology-aware CSI driver) if the data needs to survive the pod moving to a different node. "+
				"If hostPath is intentional (e.g. accessing node-local state by design), document that this workload is pinned to its current node and can't be safely drained without manual coordination.",
		))
	}
	if len(localPVs) > 0 {
		out = append(out, drain002LocalPVFinding(snap, kind, meta, localPVs, singleton, targetVersion))
	}
	return out, true
}

// drain002LocalPVFinding aggregates every matched node-pinned PV into one
// finding per controller (StatefulSets synthesize one PVC per ordinal, so
// a multi-replica StatefulSet naturally resolves to multiple entries here
// -- these must collapse into one finding, not one per ordinal, per the
// "dedupe to controller level" false-positive guard). Escalates to Blocker
// only if singleton (so there's exactly one entry) and its pinned node is
// currently NotReady.
func drain002LocalPVFinding(snap *k8s.Snapshot, kind string, meta metav1.ObjectMeta, localPVs []drain002LocalPV, singleton bool, targetVersion string) findings.Finding {
	sort.Slice(localPVs, func(i, j int) bool { return localPVs[i].claimName < localPVs[j].claimName })

	severity := findings.SeverityWarning
	var notes []string
	evidence := make([]string, 0, len(localPVs)*2)
	pvNames := make([]string, 0, len(localPVs))
	for _, lp := range localPVs {
		s, note := drain002Severity(snap, singleton, lp.nodeName)
		if s == findings.SeverityBlocker {
			severity = findings.SeverityBlocker
			notes = append(notes, note)
		}
		evidence = append(evidence,
			fmt.Sprintf("PersistentVolumeClaim: %s -> PersistentVolume: %s (pinned node: %s)", lp.claimName, lp.pvName, drain002NodeLabel(lp.nodeName)))
		pvNames = append(pvNames, lp.pvName)
	}
	note := strings.Join(notes, "")

	msg := fmt.Sprintf(
		"%s %s/%s uses %d PersistentVolumeClaim(s) bound to node-pinned PersistentVolume(s) — evicting the affected pod(s) requires the replacement pod to land back on the exact same node%s",
		kind, meta.Namespace, meta.Name, len(localPVs), note)

	return drain002Finding(kind, meta, "local-pv:"+strings.Join(pvNames, ","), severity, targetVersion, msg, evidence,
		"Migrate to a networked/replicated storage class if this data needs to survive the node being removed. "+
			"If node-local storage is intentional, document that this workload can't be safely drained from its pinned node(s) without manual data migration.",
	)
}

// drain002Severity applies the design doc's Blocker escalation: only when
// the exact node is known, that node is currently NotReady, and this is a
// singleton (no other replica could plausibly be serving in its place).
// Multi-replica StatefulSets never escalate here -- reasoning correctly
// about whether other ordinals substitute for one pinned replica's data is
// genuinely ambiguous, so this stays Warning rather than risk a false
// Blocker.
func drain002Severity(snap *k8s.Snapshot, singleton bool, nodeName string) (findings.Severity, string) {
	if !singleton || nodeName == "" {
		return findings.SeverityWarning, ""
	}
	for _, node := range snap.Nodes {
		if node.Name != nodeName {
			continue
		}
		if !nodeIsReady(node) {
			return findings.SeverityBlocker, fmt.Sprintf(" — node %q is currently NotReady, so this is not a future risk but a present one", nodeName)
		}
		break
	}
	return findings.SeverityWarning, ""
}

func nodeIsReady(node corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

func drain002NodeLabel(nodeName string) string {
	if nodeName == "" {
		return "unknown (could not be determined from current state)"
	}
	return nodeName
}

// statefulSetSyntheticPVCVolumes reconstructs the PersistentVolumeClaim
// volumes the StatefulSet controller generates per pod ordinal, one per
// (volumeClaimTemplate, ordinal) pair, named "<template>-<statefulset>-
// <ordinal>" -- the same deterministic naming StatefulSet itself uses. An
// unset spec.replicas defaults to 1, matching isSingletonReplicaCount's
// own default-handling.
func statefulSetSyntheticPVCVolumes(sts appsv1.StatefulSet) []corev1.Volume {
	replicas := int32(1)
	if sts.Spec.Replicas != nil {
		replicas = *sts.Spec.Replicas
	}
	var volumes []corev1.Volume
	for _, tmpl := range sts.Spec.VolumeClaimTemplates {
		for ordinal := int32(0); ordinal < replicas; ordinal++ {
			claimName := fmt.Sprintf("%s-%s-%d", tmpl.Name, sts.Name, ordinal)
			volumes = append(volumes, corev1.Volume{
				Name:         tmpl.Name,
				VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claimName}},
			})
		}
	}
	return volumes
}

// drain002HostPathVolumes returns the volume names in podSpec that use a
// hostPath source.
func drain002HostPathVolumes(podSpec corev1.PodSpec) []string {
	var names []string
	for _, v := range podSpec.Volumes {
		if v.HostPath != nil {
			names = append(names, v.Name)
		}
	}
	sort.Strings(names)
	return names
}

type drain002LocalPV struct {
	claimName, pvName, nodeName string
}

// drain002LocalPVVolumes resolves each PVC-backed volume in podSpec to its
// bound PersistentVolume and reports the ones pinned to one exact node.
func drain002LocalPVVolumes(snap *k8s.Snapshot, namespace string, podSpec corev1.PodSpec) []drain002LocalPV {
	var out []drain002LocalPV
	for _, v := range podSpec.Volumes {
		if v.PersistentVolumeClaim == nil {
			continue
		}
		pvc := findPVC(snap, namespace, v.PersistentVolumeClaim.ClaimName)
		if pvc == nil || pvc.Spec.VolumeName == "" {
			continue
		}
		pv := findPV(snap, pvc.Spec.VolumeName)
		if pv == nil {
			continue
		}
		if nodeName, pinned := pvPinnedNodeName(*pv); pinned {
			out = append(out, drain002LocalPV{claimName: v.PersistentVolumeClaim.ClaimName, pvName: pv.Name, nodeName: nodeName})
		}
	}
	return out
}

func findPVC(snap *k8s.Snapshot, namespace, name string) *corev1.PersistentVolumeClaim {
	for i := range snap.PersistentVolumeClaims {
		pvc := &snap.PersistentVolumeClaims[i]
		if pvc.Namespace == namespace && pvc.Name == name {
			return pvc
		}
	}
	return nil
}

func findPV(snap *k8s.Snapshot, name string) *corev1.PersistentVolume {
	for i := range snap.PersistentVolumes {
		if snap.PersistentVolumes[i].Name == name {
			return &snap.PersistentVolumes[i]
		}
	}
	return nil
}

// pvPinnedNodeName reports whether pv is deterministically bound to one
// exact node: either the Local volume source (which the API only allows
// with nodeAffinity set), or any PV whose required nodeAffinity has a
// kubernetes.io/hostname In-selector with exactly one value. Deliberately
// excludes broader topology affinity (zone/region, or multiple hostname
// values) -- that's normal for network-attached CSI storage and is not
// "stuck to one node" the way this rule means it.
func pvPinnedNodeName(pv corev1.PersistentVolume) (nodeName string, pinned bool) {
	if pv.Spec.NodeAffinity == nil || pv.Spec.NodeAffinity.Required == nil {
		return "", pv.Spec.Local != nil
	}
	for _, term := range pv.Spec.NodeAffinity.Required.NodeSelectorTerms {
		for _, expr := range term.MatchExpressions {
			if expr.Key == drainHostnameLabel && expr.Operator == corev1.NodeSelectorOpIn && len(expr.Values) == 1 {
				return expr.Values[0], true
			}
		}
	}
	return "", pv.Spec.Local != nil
}

// drain002RunningNodeName finds the node a hostPath pod is currently
// scheduled on, via the workload's own label selector match against live
// pods -- used only to give the Blocker-escalation check something to
// correlate against, not as a scheduling guarantee. Returns "" if no
// currently-scheduled pod is found (e.g. Pending).
func drain002RunningNodeName(snap *k8s.Snapshot, namespace, name, kind string) string {
	for _, pod := range snap.Pods {
		if pod.Namespace != namespace || pod.Spec.NodeName == "" || !isLiveWorkloadPod(pod) {
			continue
		}
		for _, owner := range pod.OwnerReferences {
			if ownerMatchesWorkload(owner, kind, name) {
				return pod.Spec.NodeName
			}
			// Deployment pods are owned by an intermediate ReplicaSet, not
			// the Deployment directly -- match the ReplicaSet's name
			// prefix as a best-effort fallback rather than resolving the
			// full owner chain (no ReplicaSet collection exists in
			// Snapshot today).
			if kind == "Deployment" && owner.Kind == "ReplicaSet" && strings.HasPrefix(owner.Name, name+"-") {
				return pod.Spec.NodeName
			}
		}
	}
	return ""
}

func ownerMatchesWorkload(owner metav1.OwnerReference, kind, name string) bool {
	return owner.Kind == kind && owner.Name == name
}

func drain002Finding(kind string, meta metav1.ObjectMeta, discriminator string, severity findings.Severity, targetVersion, message string, evidence []string, remediation string) findings.Finding {
	ref := findings.LiveResource(kind, findings.ScopeNamespaced, meta.Namespace, meta.Name, string(meta.UID))
	return findings.Finding{
		RuleID:      "DRAIN-002",
		Severity:    severity,
		Confidence:  findings.TierObserved,
		Message:     message,
		Resources:   []findings.ResourceReference{ref},
		Evidence:    evidence,
		Remediation: remediation,
		Fingerprint: findings.FingerprintV2("DRAIN-002", targetVersion, discriminator, ref),
	}
}
