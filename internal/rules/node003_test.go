package rules

import (
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"kubepreflight/internal/collectors/k8s"
	"kubepreflight/internal/collectors/manifest"
	"kubepreflight/internal/findings"
)

func node003Deployment(namespace, name string, spec corev1.PodSpec) appsv1.Deployment {
	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, UID: types.UID("uid-" + name)},
		Spec:       appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: spec}},
	}
}

func node003ManifestWorkload(kind, namespace, name, sourcePath, podSpecPath string, podSpec map[string]interface{}) manifest.WorkloadObject {
	return manifest.WorkloadObject{
		Kind:        kind,
		Namespace:   namespace,
		Name:        name,
		SourcePath:  sourcePath,
		PodSpec:     podSpec,
		PodSpecPath: podSpecPath,
	}
}

func TestNODE003_Positive_NodeSelectorOnAppWorkloadIsWarning(t *testing.T) {
	snap := &k8s.Snapshot{Errors: map[string]error{}, Deployments: []appsv1.Deployment{
		node003Deployment("payments", "legacy-pinned", corev1.PodSpec{
			NodeSelector: map[string]string{"node-role.kubernetes.io/master": ""},
		}),
	}}

	fs, err := (NODE003{}).Evaluate(&ScanContext{K8s: snap}, "1.36")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
	f := fs[0]
	if f.Severity != findings.SeverityWarning || f.CriticalInfra {
		t.Errorf("app workload = severity %s, criticalInfra %v — want Warning, false", f.Severity, f.CriticalInfra)
	}
	if !strings.Contains(strings.Join(f.Evidence, "\n"), `spec.template.spec.nodeSelector["node-role.kubernetes.io/master"]`) {
		t.Errorf("evidence missing the exact nodeSelector path: %v", f.Evidence)
	}
	if !strings.Contains(f.Remediation, "node-role.kubernetes.io/control-plane") {
		t.Errorf("remediation must name the replacement label: %q", f.Remediation)
	}
}

func TestNODE003_Positive_KubeSystemEscalatesToBlockerCriticalInfra(t *testing.T) {
	snap := &k8s.Snapshot{Errors: map[string]error{}, DaemonSets: []appsv1.DaemonSet{{
		ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: "some-agent", UID: "uid-agent"},
		Spec: appsv1.DaemonSetSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			Affinity: &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{{
						MatchExpressions: []corev1.NodeSelectorRequirement{{Key: "node-role.kubernetes.io/master", Operator: corev1.NodeSelectorOpExists}},
					}},
				},
			}},
		}}},
	}}}

	fs, err := (NODE003{}).Evaluate(&ScanContext{K8s: snap}, "1.36")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
	f := fs[0]
	if f.Severity != findings.SeverityBlocker || !f.CriticalInfra {
		t.Errorf("kube-system workload = severity %s, criticalInfra %v — want Blocker, true", f.Severity, f.CriticalInfra)
	}
	wantPath := "spec.template.spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].key"
	if !strings.Contains(strings.Join(f.Evidence, "\n"), wantPath) {
		t.Errorf("evidence missing the exact affinity path %q: %v", wantPath, f.Evidence)
	}
}

func TestNODE003_Positive_CriticalNameOutsideKubeSystemEscalates(t *testing.T) {
	snap := &k8s.Snapshot{Errors: map[string]error{}, DaemonSets: []appsv1.DaemonSet{{
		ObjectMeta: metav1.ObjectMeta{Namespace: "networking", Name: "calico-node", UID: "uid-calico"},
		Spec: appsv1.DaemonSetSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			Tolerations: []corev1.Toleration{{Key: "node-role.kubernetes.io/master", Operator: corev1.TolerationOpExists}},
		}}},
	}}}

	fs, err := (NODE003{}).Evaluate(&ScanContext{K8s: snap}, "1.36")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 || fs[0].Severity != findings.SeverityBlocker || !fs[0].CriticalInfra {
		t.Fatalf("calico-node outside kube-system must still escalate: %+v", fs)
	}
	if !strings.Contains(strings.Join(fs[0].Evidence, "\n"), "spec.template.spec.tolerations[0].key") {
		t.Errorf("evidence missing the toleration path: %v", fs[0].Evidence)
	}
}

func TestNODE003_Positive_PreferredAffinityAndMultiplePathsMergeIntoOneFinding(t *testing.T) {
	snap := &k8s.Snapshot{Errors: map[string]error{}, Deployments: []appsv1.Deployment{
		node003Deployment("tools", "ops-dashboard", corev1.PodSpec{
			NodeSelector: map[string]string{"node-role.kubernetes.io/master": ""},
			Affinity: &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{{
					Weight:     1,
					Preference: corev1.NodeSelectorTerm{MatchExpressions: []corev1.NodeSelectorRequirement{{Key: "node-role.kubernetes.io/master", Operator: corev1.NodeSelectorOpExists}}},
				}},
			}},
		}),
	}}

	fs, err := (NODE003{}).Evaluate(&ScanContext{K8s: snap}, "1.36")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("multiple paths on one workload must merge into one finding, got %d: %+v", len(fs), fs)
	}
	joined := strings.Join(fs[0].Evidence, "\n")
	if !strings.Contains(joined, "nodeSelector") || !strings.Contains(joined, "preferredDuringSchedulingIgnoredDuringExecution[0].preference.matchExpressions[0].key") {
		t.Errorf("evidence must list every matched path: %v", fs[0].Evidence)
	}
	if len(fs[0].RemediationDetail.Changes) != 2 {
		t.Errorf("Changes = %d entries, want one per matched path (2): %+v", len(fs[0].RemediationDetail.Changes), fs[0].RemediationDetail.Changes)
	}
}

func TestNODE003_Negative_ControlPlaneLabelAndUnrelatedSelectorsNoFinding(t *testing.T) {
	snap := &k8s.Snapshot{Errors: map[string]error{}, Deployments: []appsv1.Deployment{
		node003Deployment("kube-system", "modern-agent", corev1.PodSpec{
			NodeSelector: map[string]string{"node-role.kubernetes.io/control-plane": ""},
			Tolerations:  []corev1.Toleration{{Key: "node-role.kubernetes.io/control-plane", Operator: corev1.TolerationOpExists}},
		}),
		node003Deployment("payments", "normal-app", corev1.PodSpec{
			NodeSelector: map[string]string{"kubernetes.io/arch": "amd64"},
		}),
	}}

	fs, err := (NODE003{}).Evaluate(&ScanContext{K8s: snap}, "1.36")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("control-plane label and unrelated selectors must not fire, got %d: %+v", len(fs), fs)
	}
}

func TestNODE003_Negative_NilSnapshotNoFindingsNoError(t *testing.T) {
	fs, err := (NODE003{}).Evaluate(&ScanContext{K8s: nil}, "1.36")
	if err != nil || len(fs) != 0 {
		t.Fatalf("nil snapshot must be a no-op, got %d findings, err %v", len(fs), err)
	}
}

// TestNODE003_EndToEnd_PriorityThroughNewReport proves the escalation
// reaches the report layer: a plain app hit lands at P4/workload/
// continue=true, and a kube-system hit at P2/cluster/continue=false —
// through the real registry and NewReport, not AssignPriority in isolation.
func TestNODE003_EndToEnd_PriorityThroughNewReport(t *testing.T) {
	snap := &k8s.Snapshot{Errors: map[string]error{}, Deployments: []appsv1.Deployment{
		node003Deployment("payments", "legacy-pinned", corev1.PodSpec{NodeSelector: map[string]string{"node-role.kubernetes.io/master": ""}}),
		node003Deployment("kube-system", "critical-agent", corev1.PodSpec{NodeSelector: map[string]string{"node-role.kubernetes.io/master": ""}}),
	}}
	fs, err := NewDefaultRegistry().RunAll(&ScanContext{K8s: snap}, "1.36")
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	r := findings.NewReport("1.36", "test", "", metav1.Now().Time, fs)

	byNS := map[string]findings.Finding{}
	for _, f := range r.Findings {
		if f.RuleID == "NODE-003" {
			byNS[f.Resources[0].Namespace] = f
		}
	}
	if len(byNS) != 2 {
		t.Fatalf("want NODE-003 findings for both namespaces, got %+v", byNS)
	}
	app := byNS["payments"]
	if app.Priority != "P4" || app.AffectedScope != "workload" || !app.CanUpgradeContinue {
		t.Errorf("app finding = %s/%s continue=%v, want P4/workload continue=true", app.Priority, app.AffectedScope, app.CanUpgradeContinue)
	}
	sys := byNS["kube-system"]
	if sys.Priority != "P2" || sys.AffectedScope != "cluster" || sys.CanUpgradeContinue {
		t.Errorf("kube-system finding = %s/%s continue=%v, want P2/cluster continue=false", sys.Priority, sys.AffectedScope, sys.CanUpgradeContinue)
	}
}

func TestNODE003_ManifestDeploymentNodeSelectorIsWarningP4(t *testing.T) {
	snap := &manifest.Snapshot{Errors: map[string]error{}, Workloads: []manifest.WorkloadObject{
		node003ManifestWorkload("Deployment", "apps", "api", "manifests/workloads/api.yaml", "spec.template.spec", map[string]interface{}{
			"nodeSelector": map[string]interface{}{deprecatedMasterNodeLabel: ""},
		}),
	}}

	fs, err := (NODE003{}).Evaluate(&ScanContext{Manifests: snap}, "1.36")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
	f := findings.NewReport("1.36", "test", "", metav1.Now().Time, fs).Findings[0]
	if f.Severity != findings.SeverityWarning || f.Priority != "P4" || f.AffectedScope != "workload" || !f.CanUpgradeContinue {
		t.Fatalf("manifest Deployment = severity=%s priority=%s scope=%s continue=%v, want Warning/P4/workload/true", f.Severity, f.Priority, f.AffectedScope, f.CanUpgradeContinue)
	}
	if f.Resources[0].Plane != findings.PlaneManifest || f.Resources[0].SourcePath != "manifests/workloads/api.yaml" {
		t.Fatalf("resource = %+v, want manifest source path", f.Resources[0])
	}
	evidence := strings.Join(f.Evidence, "\n")
	for _, want := range []string{
		"manifests/workloads/api.yaml",
		"Deployment apps/api",
		deprecatedMasterNodeLabel,
		"spec.template.spec.nodeSelector",
	} {
		if !strings.Contains(evidence, want) {
			t.Errorf("evidence missing %q: %v", want, f.Evidence)
		}
	}
}

func TestNODE003_ManifestDaemonSetKubeSystemEscalatesToBlockerP2(t *testing.T) {
	snap := &manifest.Snapshot{Errors: map[string]error{}, Workloads: []manifest.WorkloadObject{
		node003ManifestWorkload("DaemonSet", "kube-system", "network-agent", "daemonset.yaml", "spec.template.spec", map[string]interface{}{
			"nodeSelector": map[string]interface{}{deprecatedMasterNodeLabel: ""},
		}),
	}}

	fs, err := (NODE003{}).Evaluate(&ScanContext{Manifests: snap}, "1.36")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
	f := findings.NewReport("1.36", "test", "", metav1.Now().Time, fs).Findings[0]
	if f.Severity != findings.SeverityBlocker || f.Priority != "P2" || f.AffectedScope != "cluster" || f.CanUpgradeContinue || !f.CriticalInfra {
		t.Fatalf("kube-system DaemonSet = severity=%s priority=%s scope=%s continue=%v critical=%v, want Blocker/P2/cluster/false/true", f.Severity, f.Priority, f.AffectedScope, f.CanUpgradeContinue, f.CriticalInfra)
	}
}

func TestNODE003_ManifestStatefulSetRequiredAffinity(t *testing.T) {
	snap := &manifest.Snapshot{Errors: map[string]error{}, Workloads: []manifest.WorkloadObject{
		node003ManifestWorkload("StatefulSet", "data", "store", "statefulset.yaml", "spec.template.spec", map[string]interface{}{
			"affinity": map[string]interface{}{"nodeAffinity": map[string]interface{}{
				"requiredDuringSchedulingIgnoredDuringExecution": map[string]interface{}{
					"nodeSelectorTerms": []interface{}{map[string]interface{}{
						"matchExpressions": []interface{}{map[string]interface{}{"key": deprecatedMasterNodeLabel, "operator": "Exists"}},
					}},
				},
			}},
		}),
	}}

	fs, err := (NODE003{}).Evaluate(&ScanContext{Manifests: snap}, "1.36")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
	if got := strings.Join(fs[0].Evidence, "\n"); !strings.Contains(got, "requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].key") {
		t.Fatalf("evidence missing required affinity path: %v", fs[0].Evidence)
	}
}

func TestNODE003_ManifestCronJobPreferredAffinity(t *testing.T) {
	snap := &manifest.Snapshot{Errors: map[string]error{}, Workloads: []manifest.WorkloadObject{
		node003ManifestWorkload("CronJob", "batch", "cleanup", "cronjob.yaml", "spec.jobTemplate.spec.template.spec", map[string]interface{}{
			"affinity": map[string]interface{}{"nodeAffinity": map[string]interface{}{
				"preferredDuringSchedulingIgnoredDuringExecution": []interface{}{map[string]interface{}{
					"weight": float64(1),
					"preference": map[string]interface{}{
						"matchExpressions": []interface{}{map[string]interface{}{"key": deprecatedMasterNodeLabel, "operator": "Exists"}},
					},
				}},
			}},
		}),
	}}

	fs, err := (NODE003{}).Evaluate(&ScanContext{Manifests: snap}, "1.36")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
	if got := strings.Join(fs[0].Evidence, "\n"); !strings.Contains(got, "spec.jobTemplate.spec.template.spec.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].preference.matchExpressions[0].key") {
		t.Fatalf("evidence missing preferred affinity path: %v", fs[0].Evidence)
	}
}

func TestNODE003_ManifestPodToleration(t *testing.T) {
	snap := &manifest.Snapshot{Errors: map[string]error{}, Workloads: []manifest.WorkloadObject{
		node003ManifestWorkload("Pod", "apps", "singleton", "pod.yaml", "spec", map[string]interface{}{
			"tolerations": []interface{}{map[string]interface{}{"key": deprecatedMasterNodeLabel, "operator": "Exists"}},
		}),
	}}

	fs, err := (NODE003{}).Evaluate(&ScanContext{Manifests: snap}, "1.36")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
	if got := strings.Join(fs[0].Evidence, "\n"); !strings.Contains(got, "spec.tolerations[0].key") {
		t.Fatalf("evidence missing pod toleration path: %v", fs[0].Evidence)
	}
}

func TestNODE003_ManifestControlPlaneLabelOnlyNoFinding(t *testing.T) {
	snap := &manifest.Snapshot{Errors: map[string]error{}, Workloads: []manifest.WorkloadObject{
		node003ManifestWorkload("Deployment", "apps", "modern", "deployment.yaml", "spec.template.spec", map[string]interface{}{
			"nodeSelector": map[string]interface{}{replacementControlPlaneLabel: ""},
			"tolerations":  []interface{}{map[string]interface{}{"key": replacementControlPlaneLabel, "operator": "Exists"}},
		}),
	}}

	fs, err := (NODE003{}).Evaluate(&ScanContext{Manifests: snap}, "1.36")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("control-plane label only must not fire, got %d: %+v", len(fs), fs)
	}
}
