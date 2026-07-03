package rules

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"kubepreflight/internal/collectors/k8s"
	"kubepreflight/internal/findings"
)

// PDB002 flags two or more PodDisruptionBudgets in the same namespace whose
// selectors both match at least one live pod: the Eviction API rejects
// eviction when multiple PDBs match a pod, even if each individually would
// allow disruption. This is the general form of the documented AWS-managed
// CoreDNS duplicate-PDB trap — no CoreDNS-specific code, it's just this rule
// firing in kube-system (deep dive Section 6, check PDB-002).
type PDB002 struct{}

func (PDB002) ID() string { return "PDB-002" }

func (PDB002) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	snap := sc.K8s
	var out []findings.Finding

	pdbs := snap.PodDisruptionBudgets
	for i := 0; i < len(pdbs); i++ {
		for j := i + 1; j < len(pdbs); j++ {
			a, b := pdbs[i], pdbs[j]
			if a.Namespace != b.Namespace {
				continue
			}

			selA, err := metav1.LabelSelectorAsSelector(a.Spec.Selector)
			if err != nil || selA.Empty() {
				continue
			}
			selB, err := metav1.LabelSelectorAsSelector(b.Spec.Selector)
			if err != nil || selB.Empty() {
				continue
			}

			overlapping := overlappingPodNames(snap, a.Namespace, selA, selB)
			if len(overlapping) == 0 {
				continue
			}
			out = append(out, pdb002Finding(a.Namespace, a.Name, string(a.UID), selA.String(),
				b.Name, string(b.UID), selB.String(), overlapping, targetVersion))
		}
	}

	return out, nil
}

func overlappingPodNames(snap *k8s.Snapshot, namespace string, selA, selB labels.Selector) []string {
	var names []string
	for _, pod := range snap.Pods {
		if pod.Namespace != namespace {
			continue
		}
		podLabels := labels.Set(pod.Labels)
		if selA.Matches(podLabels) && selB.Matches(podLabels) {
			names = append(names, pod.Name)
		}
	}
	return names
}

func pdb002Finding(namespace, nameA, uidA, selectorA, nameB, uidB, selectorB string, overlappingPods []string, targetVersion string) findings.Finding {
	msg := fmt.Sprintf(
		"PodDisruptionBudgets %s/%s and %s/%s select an overlapping set of pods (%d overlapping: %s) — the Eviction API rejects eviction when multiple PDBs match the same pod, even if each individually would allow disruption",
		namespace, nameA, namespace, nameB, len(overlappingPods), strings.Join(overlappingPods, ", "))

	remediation := "Overlap is always a misconfiguration: delete the duplicate/redundant PDB, or narrow one selector so the two budgets no longer target the same pods. " +
		"If this is the AWS-managed CoreDNS PDB colliding with a hand-created duplicate in kube-system, delete the duplicate and keep the AWS-managed one."

	refA := findings.LiveResource("PodDisruptionBudget", findings.ScopeNamespaced, namespace, nameA, uidA)
	refB := findings.LiveResource("PodDisruptionBudget", findings.ScopeNamespaced, namespace, nameB, uidB)
	refs := []findings.ResourceReference{refA, refB}
	return findings.Finding{
		RuleID:     "PDB-002",
		Severity:   findings.SeverityBlocker,
		Confidence: findings.TierStaticCertain,
		Message:    msg,
		Resources:  refs,
		Evidence: []string{
			fmt.Sprintf("PDB A: %s/%s (selector: %s)", namespace, nameA, selectorA),
			fmt.Sprintf("PDB B: %s/%s (selector: %s)", namespace, nameB, selectorB),
			fmt.Sprintf("overlapping pods: %s", strings.Join(overlappingPods, ", ")),
		},
		Remediation: remediation,
		Fingerprint: findings.FingerprintV2("PDB-002", targetVersion, "", refs...),
	}
}
