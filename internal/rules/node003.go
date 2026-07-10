package rules

import (
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubepreflight/internal/collectors/manifest"
	"kubepreflight/internal/findings"
)

// deprecatedMasterNodeLabel is the pre-1.24 kubeadm control-plane node role
// label (and matching taint key). kubeadm stopped adding it to new
// control-plane nodes in Kubernetes 1.24 — new/rebuilt nodes carry only
// node-role.kubernetes.io/control-plane — so a workload that still selects
// on the old key can silently stop scheduling after a control-plane node
// rebuild, cluster replacement, or platform label cleanup.
const deprecatedMasterNodeLabel = "node-role.kubernetes.io/master"

// replacementControlPlaneLabel is the current, official control-plane node
// role label per the Kubernetes well-known labels registry.
const replacementControlPlaneLabel = "node-role.kubernetes.io/control-plane"

// node003CriticalComponentNames are workload names (any namespace) treated
// as cluster-critical infrastructure for escalation, alongside the crisp
// namespace rule (anything in kube-system). Deliberately a small,
// exact-match list — CNI, DNS, service proxy, autoscaler — not a fuzzy
// "looks controller-adjacent" heuristic, so escalation stays testable and
// false-positive-free. Extend deliberately, with a test, per name.
var node003CriticalComponentNames = map[string]struct{}{
	"aws-node":           {},
	"calico-node":        {},
	"calico-typha":       {},
	"cilium":             {},
	"cilium-operator":    {},
	"kube-proxy":         {},
	"coredns":            {},
	"kube-dns":           {},
	"cluster-autoscaler": {},
}

// NODE003 flags workloads whose pod template still schedules against the
// deprecated node-role.kubernetes.io/master node label (nodeSelector,
// nodeAffinity, or a toleration of the matching taint key). It evaluates
// live Deployments/DaemonSets plus structured raw manifest workload specs.
// Deliberately out of scope: ConfigMap/Secret text scanning and fuzzy YAML
// string matching.
type NODE003 struct{}

func (NODE003) ID() string { return "NODE-003" }

func (NODE003) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	if sc == nil {
		return nil, nil
	}

	var out []findings.Finding
	if sc.K8s != nil {
		for _, d := range sc.K8s.Deployments {
			if paths := masterLabelRefs(d.Spec.Template.Spec); len(paths) > 0 {
				out = append(out, node003Finding("Deployment", d.ObjectMeta, paths, targetVersion))
			}
		}
		for _, ds := range sc.K8s.DaemonSets {
			if paths := masterLabelRefs(ds.Spec.Template.Spec); len(paths) > 0 {
				out = append(out, node003Finding("DaemonSet", ds.ObjectMeta, paths, targetVersion))
			}
		}
	}
	if sc.Manifests != nil {
		for _, obj := range sc.Manifests.Workloads {
			if paths := manifestMasterLabelRefs(obj.PodSpec, obj.PodSpecPath); len(paths) > 0 {
				out = append(out, node003ManifestFinding(obj, paths, targetVersion))
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return node003SortKey(out[i].Resources[0]) < node003SortKey(out[j].Resources[0])
	})
	return out, nil
}

// masterLabelRefs returns the pod-template spec paths (rooted at
// "spec.template.spec.") that reference the deprecated master node label,
// covering nodeSelector keys, required and preferred nodeAffinity term
// keys (matchExpressions and matchFields), and toleration keys.
func masterLabelRefs(spec corev1.PodSpec) []string {
	var paths []string
	if _, ok := spec.NodeSelector[deprecatedMasterNodeLabel]; ok {
		paths = append(paths, fmt.Sprintf("spec.template.spec.nodeSelector[%q]", deprecatedMasterNodeLabel))
	}
	if spec.Affinity != nil && spec.Affinity.NodeAffinity != nil {
		na := spec.Affinity.NodeAffinity
		if na.RequiredDuringSchedulingIgnoredDuringExecution != nil {
			for ti, term := range na.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
				paths = append(paths, nodeSelectorTermRefs(term,
					fmt.Sprintf("spec.template.spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[%d]", ti))...)
			}
		}
		for pi, pref := range na.PreferredDuringSchedulingIgnoredDuringExecution {
			paths = append(paths, nodeSelectorTermRefs(pref.Preference,
				fmt.Sprintf("spec.template.spec.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[%d].preference", pi))...)
		}
	}
	for i, tol := range spec.Tolerations {
		if tol.Key == deprecatedMasterNodeLabel {
			paths = append(paths, fmt.Sprintf("spec.template.spec.tolerations[%d].key", i))
		}
	}
	return paths
}

func nodeSelectorTermRefs(term corev1.NodeSelectorTerm, prefix string) []string {
	var paths []string
	for i, expr := range term.MatchExpressions {
		if expr.Key == deprecatedMasterNodeLabel {
			paths = append(paths, fmt.Sprintf("%s.matchExpressions[%d].key", prefix, i))
		}
	}
	for i, field := range term.MatchFields {
		if field.Key == deprecatedMasterNodeLabel {
			paths = append(paths, fmt.Sprintf("%s.matchFields[%d].key", prefix, i))
		}
	}
	return paths
}

// node003Critical reports whether the workload counts as cluster-critical
// infrastructure for escalation: crisp rules only — the kube-system
// namespace, an exact name match, or an exact known-critical label value.
func node003Critical(namespace, name string) bool {
	return node003CriticalWithLabels(namespace, name, nil)
}

func node003CriticalWithLabels(namespace, name string, labels map[string]string) bool {
	if namespace == "kube-system" {
		return true
	}
	if _, ok := node003CriticalComponentNames[name]; ok {
		return true
	}
	for _, value := range labels {
		if _, ok := node003CriticalComponentNames[value]; ok {
			return true
		}
	}
	return false
}

func node003Finding(kind string, meta metav1.ObjectMeta, paths []string, targetVersion string) findings.Finding {
	critical := node003CriticalWithLabels(meta.Namespace, meta.Name, meta.Labels)
	severity := findings.SeverityWarning
	if critical {
		severity = findings.SeverityBlocker
	}

	msg := fmt.Sprintf(
		"%s %s/%s schedules using the deprecated %s node label — new control-plane nodes carry %s instead, so this workload may fail to schedule after a control-plane node rebuild, cluster replacement, or platform label cleanup",
		kind, meta.Namespace, meta.Name, deprecatedMasterNodeLabel, replacementControlPlaneLabel)
	if critical {
		msg = fmt.Sprintf(
			"Critical component %s %s/%s depends on the deprecated %s node label — it may fail to schedule after a control-plane node rebuild or upgrade, taking cluster infrastructure down with it",
			kind, meta.Namespace, meta.Name, deprecatedMasterNodeLabel)
	}

	evidence := make([]string, 0, len(paths)+1)
	for _, p := range paths {
		evidence = append(evidence, "references "+deprecatedMasterNodeLabel+" at "+p)
	}
	evidence = append(evidence, "replacement label: "+replacementControlPlaneLabel+" (kubeadm stopped adding the master label to new control-plane nodes in Kubernetes 1.24)")

	remediation := "Replace deprecated " + deprecatedMasterNodeLabel + " references with " + replacementControlPlaneLabel + ", or migrate to an explicit stable node label managed by the platform team. " +
		"Validate that all target nodes already carry the replacement label before changing selectors or affinities — changing the selector first strands the workload with no schedulable nodes."

	changes := make([]findings.RemediationChange, 0, len(paths))
	for _, p := range paths {
		changes = append(changes, findings.RemediationChange{Field: p, Current: deprecatedMasterNodeLabel, Required: replacementControlPlaneLabel + " (after confirming nodes carry it)"})
	}

	kindLower := strings.ToLower(kind)
	ref := findings.LiveResource(kind, findings.ScopeNamespaced, meta.Namespace, meta.Name, string(meta.UID))
	return findings.Finding{
		RuleID:        "NODE-003",
		Severity:      severity,
		Confidence:    findings.TierStaticCertain,
		Message:       msg,
		Resources:     []findings.ResourceReference{ref},
		Evidence:      evidence,
		Remediation:   remediation,
		CriticalInfra: critical,
		RemediationDetail: &findings.RemediationDetail{
			Changes: changes,
			SafeFix: &findings.RemediationAction{
				Label: "Safe fix",
				Steps: []string{
					"Confirm which node role labels the target nodes actually carry before touching any selector or affinity.",
					"If migrating to a platform-owned custom label, apply and document it on the nodes first, then update the workload.",
				},
				Command: "kubectl get nodes --show-labels | grep -E 'node-role.kubernetes.io/(master|control-plane)'",
			},
			VerifyCommand:  fmt.Sprintf("kubectl get %s %s -n %s -o yaml", kindLower, shellQuote(meta.Name), shellQuote(meta.Namespace)),
			ExpectedResult: "no references to " + deprecatedMasterNodeLabel + " remain in the pod template",
		},
		Fingerprint: findings.FingerprintV2("NODE-003", targetVersion, "", ref),
	}
}

func node003ManifestFinding(obj manifest.WorkloadObject, paths []string, targetVersion string) findings.Finding {
	critical := node003CriticalWithLabels(obj.Namespace, obj.Name, obj.Labels)
	severity := findings.SeverityWarning
	if critical {
		severity = findings.SeverityBlocker
	}

	resourceLabel := node003ResourceLabel(obj.Namespace, obj.Name)
	msg := fmt.Sprintf(
		"%s %s in %s schedules using the deprecated %s node label — new control-plane nodes carry %s instead, so this workload may fail to schedule after a control-plane node rebuild, cluster replacement, or platform label cleanup",
		obj.Kind, resourceLabel, obj.SourcePath, deprecatedMasterNodeLabel, replacementControlPlaneLabel)
	if critical {
		msg = fmt.Sprintf(
			"Critical component %s %s in %s depends on the deprecated %s node label — it may fail to schedule after a control-plane node rebuild or upgrade, taking cluster infrastructure down with it",
			obj.Kind, resourceLabel, obj.SourcePath, deprecatedMasterNodeLabel)
	}

	evidence := make([]string, 0, len(paths)+1)
	for _, p := range paths {
		evidence = append(evidence, fmt.Sprintf("%s: %s %s references deprecated %s at %s", obj.SourcePath, obj.Kind, resourceLabel, deprecatedMasterNodeLabel, p))
	}
	evidence = append(evidence, "replacement label: "+replacementControlPlaneLabel+" (kubeadm stopped adding the master label to new control-plane nodes in Kubernetes 1.24)")

	remediation := "Replace deprecated " + deprecatedMasterNodeLabel + " references with " + replacementControlPlaneLabel + ", or migrate to an explicit stable node label managed by the platform team. " +
		"Validate that all target nodes already carry the replacement label before changing selectors or affinities — changing the selector first strands the workload with no schedulable nodes."

	changes := make([]findings.RemediationChange, 0, len(paths))
	for _, p := range paths {
		changes = append(changes, findings.RemediationChange{Field: p, Current: deprecatedMasterNodeLabel, Required: replacementControlPlaneLabel + " (after confirming nodes carry it)"})
	}

	ref := findings.ManifestResource(obj.Kind, findings.ScopeNamespaced, obj.Namespace, obj.Name, obj.SourcePath)
	return findings.Finding{
		RuleID:        "NODE-003",
		Severity:      severity,
		Confidence:    findings.TierStaticCertain,
		Message:       msg,
		Resources:     []findings.ResourceReference{ref},
		Evidence:      evidence,
		Remediation:   remediation,
		CriticalInfra: critical,
		RemediationDetail: &findings.RemediationDetail{
			AffectedFile: obj.SourcePath,
			Changes:      changes,
			SafeFix: &findings.RemediationAction{
				Label: "Safe fix",
				Steps: []string{
					"Confirm which node role labels the target nodes actually carry before touching any selector or affinity.",
					"If migrating to a platform-owned custom label, apply and document it on the nodes first, then update the manifest.",
				},
				Command: "kubectl get nodes --show-labels | grep -E 'node-role.kubernetes.io/(master|control-plane)'",
			},
			VerifyCommand:  fmt.Sprintf("kubepreflight scan --manifests %s --target-version %s", shellQuote(obj.SourcePath), shellQuote(targetVersion)),
			ExpectedResult: "no references to " + deprecatedMasterNodeLabel + " remain in the pod template",
		},
		Fingerprint: findings.FingerprintV2("NODE-003", targetVersion, "", ref),
	}
}

func manifestMasterLabelRefs(spec map[string]interface{}, podSpecPath string) []string {
	var paths []string
	if nodeSelector, ok := stringMap(spec["nodeSelector"]); ok {
		if _, exists := nodeSelector[deprecatedMasterNodeLabel]; exists {
			paths = append(paths, podSpecPath+".nodeSelector")
		}
	}

	if affinity, ok := mapValue(spec["affinity"]); ok {
		if nodeAffinity, ok := mapValue(affinity["nodeAffinity"]); ok {
			if required, ok := mapValue(nodeAffinity["requiredDuringSchedulingIgnoredDuringExecution"]); ok {
				if terms, ok := sliceValue(required["nodeSelectorTerms"]); ok {
					for ti, term := range terms {
						if termMap, ok := mapValue(term); ok {
							paths = append(paths, manifestNodeSelectorTermRefs(termMap,
								fmt.Sprintf("%s.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[%d]", podSpecPath, ti))...)
						}
					}
				}
			}
			if preferred, ok := sliceValue(nodeAffinity["preferredDuringSchedulingIgnoredDuringExecution"]); ok {
				for pi, pref := range preferred {
					if prefMap, ok := mapValue(pref); ok {
						if preference, ok := mapValue(prefMap["preference"]); ok {
							paths = append(paths, manifestNodeSelectorTermRefs(preference,
								fmt.Sprintf("%s.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[%d].preference", podSpecPath, pi))...)
						}
					}
				}
			}
		}
	}

	if tolerations, ok := sliceValue(spec["tolerations"]); ok {
		for i, tol := range tolerations {
			tolMap, ok := mapValue(tol)
			if !ok {
				continue
			}
			if key, _ := tolMap["key"].(string); key == deprecatedMasterNodeLabel {
				paths = append(paths, fmt.Sprintf("%s.tolerations[%d].key", podSpecPath, i))
			}
		}
	}
	return paths
}

func manifestNodeSelectorTermRefs(term map[string]interface{}, prefix string) []string {
	var paths []string
	for _, field := range []string{"matchExpressions", "matchFields"} {
		values, ok := sliceValue(term[field])
		if !ok {
			continue
		}
		for i, raw := range values {
			value, ok := mapValue(raw)
			if !ok {
				continue
			}
			if key, _ := value["key"].(string); key == deprecatedMasterNodeLabel {
				paths = append(paths, fmt.Sprintf("%s.%s[%d].key", prefix, field, i))
			}
		}
	}
	return paths
}

func node003ResourceLabel(namespace, name string) string {
	if namespace == "" {
		return name
	}
	return namespace + "/" + name
}

func node003SortKey(ref findings.ResourceReference) string {
	return strings.Join([]string{string(ref.Plane), ref.SourcePath, ref.Namespace, ref.Name, ref.Kind}, "\x00")
}

func mapValue(value interface{}) (map[string]interface{}, bool) {
	out, ok := value.(map[string]interface{})
	return out, ok
}

func stringMap(value interface{}) (map[string]interface{}, bool) {
	return mapValue(value)
}

func sliceValue(value interface{}) ([]interface{}, bool) {
	out, ok := value.([]interface{})
	return out, ok
}
