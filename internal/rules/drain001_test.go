package rules

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/k8s"
	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/manifest"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

func int64Ptr(v int64) *int64 { return &v }

func drain001RequireOne(t *testing.T, fs []findings.Finding) findings.Finding {
	t.Helper()
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
	if fs[0].RuleID != "DRAIN-001" {
		t.Errorf("RuleID = %q, want DRAIN-001", fs[0].RuleID)
	}
	return fs[0]
}

func TestDRAIN001_Deployment_ExplicitSingleReplica_Warning(t *testing.T) {
	sel := &metav1.LabelSelector{MatchLabels: map[string]string{"app": "cache"}}
	snap := &k8s.Snapshot{Deployments: []appsv1.Deployment{{
		ObjectMeta: metav1.ObjectMeta{Name: "cache", Namespace: "default", UID: "uid-1"},
		Spec:       appsv1.DeploymentSpec{Replicas: int32Ptr(1), Selector: sel},
	}}}
	fs, err := (DRAIN001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain001RequireOne(t, fs)
	if f.Severity != findings.SeverityWarning {
		t.Errorf("Severity = %q, want Warning", f.Severity)
	}
	if f.Confidence != findings.TierStaticCertain {
		t.Errorf("Confidence = %q, want STATIC_CERTAIN", f.Confidence)
	}
}

func TestDRAIN001_Deployment_UnsetReplicas_DefaultsToSingleton(t *testing.T) {
	snap := &k8s.Snapshot{Deployments: []appsv1.Deployment{{
		ObjectMeta: metav1.ObjectMeta{Name: "cache", Namespace: "default", UID: "uid-1"},
		Spec:       appsv1.DeploymentSpec{},
	}}}
	fs, err := (DRAIN001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	drain001RequireOne(t, fs)
}

func TestDRAIN001_Deployment_MultiReplica_NoFinding(t *testing.T) {
	snap := &k8s.Snapshot{Deployments: []appsv1.Deployment{{
		ObjectMeta: metav1.ObjectMeta{Name: "cache", Namespace: "default", UID: "uid-1"},
		Spec:       appsv1.DeploymentSpec{Replicas: int32Ptr(3)},
	}}}
	fs, err := (DRAIN001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings for a 3-replica Deployment", fs, err)
	}
}

func TestDRAIN001_Deployment_ZeroReplicas_NoFinding(t *testing.T) {
	snap := &k8s.Snapshot{Deployments: []appsv1.Deployment{{
		ObjectMeta: metav1.ObjectMeta{Name: "cache", Namespace: "default", UID: "uid-1"},
		Spec:       appsv1.DeploymentSpec{Replicas: int32Ptr(0)},
	}}}
	fs, err := (DRAIN001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings for a scaled-to-zero Deployment (nothing to drain)", fs, err)
	}
}

func TestDRAIN001_DeletingController_NoFinding(t *testing.T) {
	now := metav1.Now()
	snap := &k8s.Snapshot{Deployments: []appsv1.Deployment{{
		ObjectMeta: metav1.ObjectMeta{Name: "cache", Namespace: "default", UID: "uid-1", DeletionTimestamp: &now},
		Spec:       appsv1.DeploymentSpec{Replicas: int32Ptr(1)},
	}}}
	fs, err := (DRAIN001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings for a Deployment being deleted", fs, err)
	}
}

func TestDRAIN001_StatefulSet_SingleReplica_Warning(t *testing.T) {
	snap := &k8s.Snapshot{StatefulSets: []appsv1.StatefulSet{{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default", UID: "uid-2"},
		Spec:       appsv1.StatefulSetSpec{Replicas: int32Ptr(1)},
	}}}
	fs, err := (DRAIN001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain001RequireOne(t, fs)
	if f.Resources[0].Kind != "StatefulSet" {
		t.Errorf("Resources[0].Kind = %q, want StatefulSet", f.Resources[0].Kind)
	}
}

func TestDRAIN001_DaemonSetsNeverEvaluated(t *testing.T) {
	snap := &k8s.Snapshot{DaemonSets: []appsv1.DaemonSet{{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "default", UID: "uid-3"},
	}}}
	fs, err := (DRAIN001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want DaemonSets never evaluated by DRAIN-001", fs, err)
	}
}

func TestDRAIN001_PDBPresence_StillWarning_NotSuppressedOrEscalated(t *testing.T) {
	sel := &metav1.LabelSelector{MatchLabels: map[string]string{"app": "cache"}}
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{{
			ObjectMeta: metav1.ObjectMeta{Name: "cache", Namespace: "default", UID: "uid-1"},
			Spec:       appsv1.DeploymentSpec{Replicas: int32Ptr(1), Selector: sel},
		}},
		Pods: []corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{Name: "cache-abc", Namespace: "default", Labels: map[string]string{"app": "cache"}},
		}},
		PodDisruptionBudgets: []policyv1.PodDisruptionBudget{{
			ObjectMeta: metav1.ObjectMeta{Name: "cache-pdb", Namespace: "default"},
			Spec:       policyv1.PodDisruptionBudgetSpec{Selector: sel},
			Status:     policyv1.PodDisruptionBudgetStatus{DisruptionsAllowed: 1},
		}},
	}
	fs, err := (DRAIN001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain001RequireOne(t, fs)
	if f.Severity != findings.SeverityWarning {
		t.Errorf("Severity = %q, want Warning even with a healthy PDB present -- the PDB doesn't close the capacity gap", f.Severity)
	}
	foundPDBEvidence := false
	for _, e := range f.Evidence {
		if e == "PodDisruptionBudget(s): cache-pdb" {
			foundPDBEvidence = true
		}
	}
	if !foundPDBEvidence {
		t.Errorf("Evidence = %+v, want it to name the matching PDB", f.Evidence)
	}
}

func TestDRAIN001_NoPDB_EvidenceSaysNone(t *testing.T) {
	snap := &k8s.Snapshot{Deployments: []appsv1.Deployment{{
		ObjectMeta: metav1.ObjectMeta{Name: "cache", Namespace: "default", UID: "uid-1"},
		Spec:       appsv1.DeploymentSpec{Replicas: int32Ptr(1)},
	}}}
	fs, err := (DRAIN001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain001RequireOne(t, fs)
	found := false
	for _, e := range f.Evidence {
		if e == "PodDisruptionBudget(s): none" {
			found = true
		}
	}
	if !found {
		t.Errorf("Evidence = %+v, want an explicit 'none' entry", f.Evidence)
	}
}

func TestDRAIN001_CompletedAndDeletingPods_ExcludedFromEvidence(t *testing.T) {
	now := metav1.Now()
	sel := &metav1.LabelSelector{MatchLabels: map[string]string{"app": "cache"}}
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{{
			ObjectMeta: metav1.ObjectMeta{Name: "cache", Namespace: "default", UID: "uid-1"},
			Spec:       appsv1.DeploymentSpec{Replicas: int32Ptr(1), Selector: sel},
		}},
		Pods: []corev1.Pod{
			{ObjectMeta: metav1.ObjectMeta{Name: "cache-live", Namespace: "default", Labels: map[string]string{"app": "cache"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "cache-succeeded", Namespace: "default", Labels: map[string]string{"app": "cache"}}, Status: corev1.PodStatus{Phase: corev1.PodSucceeded}},
			{ObjectMeta: metav1.ObjectMeta{Name: "cache-deleting", Namespace: "default", Labels: map[string]string{"app": "cache"}, DeletionTimestamp: &now}},
		},
	}
	fs, err := (DRAIN001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain001RequireOne(t, fs)
	for _, e := range f.Evidence {
		if e == "affected pod(s): cache-live" {
			return
		}
	}
	t.Errorf("Evidence = %+v, want exactly 'affected pod(s): cache-live' (succeeded/deleting pods excluded)", f.Evidence)
}

func TestDRAIN001_ManifestPlane_SingleReplica_Warning(t *testing.T) {
	sc := &ScanContext{Manifests: &manifest.Snapshot{Workloads: []manifest.WorkloadObject{
		{Kind: "Deployment", Namespace: "default", Name: "cache", SourcePath: "manifests/cache.yaml", Replicas: int64Ptr(1)},
	}}}
	fs, err := (DRAIN001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain001RequireOne(t, fs)
	if f.Resources[0].SourcePath != "manifests/cache.yaml" {
		t.Errorf("Resources[0].SourcePath = %q, want manifests/cache.yaml", f.Resources[0].SourcePath)
	}
}

func TestDRAIN001_ManifestPlane_UnsetReplicas_DefaultsToSingleton(t *testing.T) {
	sc := &ScanContext{Manifests: &manifest.Snapshot{Workloads: []manifest.WorkloadObject{
		{Kind: "StatefulSet", Namespace: "default", Name: "db", SourcePath: "manifests/db.yaml"},
	}}}
	fs, err := (DRAIN001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	drain001RequireOne(t, fs)
}

func TestDRAIN001_ManifestPlane_MultiReplica_NoFinding(t *testing.T) {
	sc := &ScanContext{Manifests: &manifest.Snapshot{Workloads: []manifest.WorkloadObject{
		{Kind: "Deployment", Namespace: "default", Name: "cache", SourcePath: "manifests/cache.yaml", Replicas: int64Ptr(3)},
	}}}
	fs, err := (DRAIN001{}).Evaluate(sc, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings for a 3-replica manifest Deployment", fs, err)
	}
}

func TestDRAIN001_ManifestPlane_DaemonSetKindIgnored(t *testing.T) {
	sc := &ScanContext{Manifests: &manifest.Snapshot{Workloads: []manifest.WorkloadObject{
		{Kind: "DaemonSet", Namespace: "default", Name: "agent", SourcePath: "manifests/agent.yaml"},
	}}}
	fs, err := (DRAIN001{}).Evaluate(sc, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want DaemonSet manifest workloads never evaluated", fs, err)
	}
}

func TestDRAIN001_NilScanContext_NoPanic(t *testing.T) {
	if _, err := (DRAIN001{}).Evaluate(nil, "1.34"); err != nil {
		t.Fatalf("Evaluate(nil): %v", err)
	}
}

func TestDRAIN001_EmptyScanContext_NoPanic(t *testing.T) {
	fs, err := (DRAIN001{}).Evaluate(&ScanContext{}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings and no panic for an empty ScanContext", fs, err)
	}
}
