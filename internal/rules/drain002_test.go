package rules

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/k8s"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

func drain002RequireN(t *testing.T, fs []findings.Finding, n int) []findings.Finding {
	t.Helper()
	if len(fs) != n {
		t.Fatalf("got %d findings, want %d: %+v", len(fs), n, fs)
	}
	for _, f := range fs {
		if f.RuleID != "DRAIN-002" {
			t.Errorf("RuleID = %q, want DRAIN-002", f.RuleID)
		}
	}
	return fs
}

func drain002Deployment(name string, replicas int32, volumes []corev1.Volume) appsv1.Deployment {
	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", UID: types.UID("uid-" + name)},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(replicas),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": name}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": name}},
				Spec:       corev1.PodSpec{Volumes: volumes},
			},
		},
	}
}

func drain002Pod(name, nodeName string, labels map[string]string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Labels: labels, OwnerReferences: []metav1.OwnerReference{
			{Kind: "ReplicaSet", Name: labels["app"] + "-abc123"},
		}},
		Spec: corev1.PodSpec{NodeName: nodeName},
	}
}

func drain002NotReadyNode(name string) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status:     corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse}}},
	}
}

func drain002ReadyNode(name string) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status:     corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}}},
	}
}

func TestDRAIN002_HostPath_Warning(t *testing.T) {
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain002Deployment("logger", 1, []corev1.Volume{
			{Name: "hostlogs", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/log"}}},
		})},
		Pods:  []corev1.Pod{drain002Pod("logger-abc123-xyz", "node-a", map[string]string{"app": "logger"})},
		Nodes: []corev1.Node{drain002ReadyNode("node-a")},
	}
	fs, err := (DRAIN002{}).Evaluate(&ScanContext{K8s: snap, UpgradeContext: findings.UpgradeContextWorkerRollout}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain002RequireN(t, fs, 1)[0]
	if f.Severity != findings.SeverityWarning {
		t.Errorf("Severity = %q, want Warning (node is Ready)", f.Severity)
	}
	if f.Confidence != findings.TierObserved {
		t.Errorf("Confidence = %q, want OBSERVED", f.Confidence)
	}
}

func TestDRAIN002_HostPath_SingletonOnNotReadyNode_Blocker(t *testing.T) {
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain002Deployment("logger", 1, []corev1.Volume{
			{Name: "hostlogs", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/log"}}},
		})},
		Pods:  []corev1.Pod{drain002Pod("logger-abc123-xyz", "node-a", map[string]string{"app": "logger"})},
		Nodes: []corev1.Node{drain002NotReadyNode("node-a")},
	}
	fs, err := (DRAIN002{}).Evaluate(&ScanContext{K8s: snap, UpgradeContext: findings.UpgradeContextWorkerRollout}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain002RequireN(t, fs, 1)[0]
	if f.Severity != findings.SeverityBlocker {
		t.Errorf("Severity = %q, want Blocker -- node is currently NotReady and this is the only replica", f.Severity)
	}
}

func TestDRAIN002_HostPath_SingletonOnNotReadyNode_ContextMatrix(t *testing.T) {
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain002Deployment("logger", 1, []corev1.Volume{
			{Name: "hostlogs", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/log"}}},
		})},
		Pods:  []corev1.Pod{drain002Pod("logger-abc123-xyz", "node-a", map[string]string{"app": "logger"})},
		Nodes: []corev1.Node{drain002NotReadyNode("node-a")},
	}
	tests := []struct {
		ctx      findings.UpgradeContext
		severity findings.Severity
		gate     findings.UpgradeGate
	}{
		{findings.UpgradeContextAuditOnly, findings.SeverityWarning, findings.UpgradeGateAllow},
		{findings.UpgradeContextControlPlaneOnly, findings.SeverityWarning, findings.UpgradeGateAllow},
		{findings.UpgradeContextWorkerRollout, findings.SeverityBlocker, findings.UpgradeGateBlock},
		{findings.UpgradeContextFullPlatformUpgrade, findings.SeverityBlocker, findings.UpgradeGateBlock},
		{findings.UpgradeContextWorkloadRestart, findings.SeverityWarning, findings.UpgradeGateOperatorDecision},
		{findings.UpgradeContextUnspecified, findings.SeverityWarning, findings.UpgradeGateOperatorDecision},
	}
	for _, tc := range tests {
		fs, err := (DRAIN002{}).Evaluate(&ScanContext{K8s: snap, UpgradeContext: tc.ctx}, "1.34")
		if err != nil {
			t.Fatalf("Evaluate(%s): %v", tc.ctx, err)
		}
		f := drain002RequireN(t, fs, 1)[0]
		if f.Severity != tc.severity || f.UpgradeGate != tc.gate {
			t.Errorf("context %s severity/gate = %q/%q, want %q/%q", tc.ctx, f.Severity, f.UpgradeGate, tc.severity, tc.gate)
		}
	}
}

func TestDRAIN002_HostPath_MultiReplica_NeverBlocker(t *testing.T) {
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain002Deployment("logger", 3, []corev1.Volume{
			{Name: "hostlogs", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/log"}}},
		})},
		Pods:  []corev1.Pod{drain002Pod("logger-abc123-xyz", "node-a", map[string]string{"app": "logger"})},
		Nodes: []corev1.Node{drain002NotReadyNode("node-a")},
	}
	fs, err := (DRAIN002{}).Evaluate(&ScanContext{K8s: snap, UpgradeContext: findings.UpgradeContextWorkerRollout}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain002RequireN(t, fs, 1)[0]
	if f.Severity != findings.SeverityWarning {
		t.Errorf("Severity = %q, want Warning -- multi-replica workloads never escalate to Blocker here", f.Severity)
	}
}

// TestDRAIN002_StatefulSet_VolumeClaimTemplates_Synthesized guards a real
// bug found via kind-cluster testing: volumeClaimTemplates never appear in
// spec.template.spec.volumes -- the StatefulSet controller synthesizes a
// PersistentVolumeClaim volume per (template, ordinal) directly onto each
// live pod, named "<template>-<statefulset>-<ordinal>". A real
// rancher.io/local-path PV (kind's default StorageClass) confirmed this:
// nodeAffinity pins to one exact kubernetes.io/hostname even though the PV
// uses spec.hostPath internally, not spec.local.
func TestDRAIN002_StatefulSet_VolumeClaimTemplates_Synthesized(t *testing.T) {
	sts := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "local-db", Namespace: "default", UID: "uid-local-db"},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(1),
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{ObjectMeta: metav1.ObjectMeta{Name: "data"}},
			},
		},
	}
	snap := &k8s.Snapshot{
		StatefulSets: []appsv1.StatefulSet{sts},
		PersistentVolumeClaims: []corev1.PersistentVolumeClaim{{
			ObjectMeta: metav1.ObjectMeta{Name: "data-local-db-0", Namespace: "default"},
			Spec:       corev1.PersistentVolumeClaimSpec{VolumeName: "pvc-local-1"},
		}},
		PersistentVolumes: []corev1.PersistentVolume{{
			ObjectMeta: metav1.ObjectMeta{Name: "pvc-local-1"},
			Spec: corev1.PersistentVolumeSpec{
				PersistentVolumeSource: corev1.PersistentVolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/local-path-provisioner/pvc-local-1"}},
				NodeAffinity: &corev1.VolumeNodeAffinity{Required: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{MatchExpressions: []corev1.NodeSelectorRequirement{{Key: "kubernetes.io/hostname", Operator: corev1.NodeSelectorOpIn, Values: []string{"kind-worker"}}}},
				}}},
			},
		}},
		Nodes: []corev1.Node{drain002ReadyNode("kind-worker")},
	}
	fs, err := (DRAIN002{}).Evaluate(&ScanContext{K8s: snap, UpgradeContext: findings.UpgradeContextWorkerRollout}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain002RequireN(t, fs, 1)[0]
	if f.Severity != findings.SeverityWarning {
		t.Errorf("Severity = %q, want Warning", f.Severity)
	}
	found := false
	for _, e := range f.Evidence {
		if e == "PersistentVolumeClaim: data-local-db-0 -> PersistentVolume: pvc-local-1 (pinned node: kind-worker)" {
			found = true
		}
	}
	if !found {
		t.Errorf("Evidence = %+v, want the synthesized ordinal-0 PVC to resolve to its bound PV", f.Evidence)
	}
}

func TestDRAIN002_StatefulSet_MultiReplicaVolumeClaimTemplates_OneFindingPerController(t *testing.T) {
	sts := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-db", Namespace: "default", UID: "uid-cluster-db"},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(3),
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{ObjectMeta: metav1.ObjectMeta{Name: "data"}},
			},
		},
	}
	pv := func(name, node string) corev1.PersistentVolume {
		return corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: corev1.PersistentVolumeSpec{
				NodeAffinity: &corev1.VolumeNodeAffinity{Required: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{MatchExpressions: []corev1.NodeSelectorRequirement{{Key: "kubernetes.io/hostname", Operator: corev1.NodeSelectorOpIn, Values: []string{node}}}},
				}}},
			},
		}
	}
	snap := &k8s.Snapshot{
		StatefulSets: []appsv1.StatefulSet{sts},
		PersistentVolumeClaims: []corev1.PersistentVolumeClaim{
			{ObjectMeta: metav1.ObjectMeta{Name: "data-cluster-db-0", Namespace: "default"}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "pv-0"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "data-cluster-db-1", Namespace: "default"}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "pv-1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "data-cluster-db-2", Namespace: "default"}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "pv-2"}},
		},
		PersistentVolumes: []corev1.PersistentVolume{pv("pv-0", "node-a"), pv("pv-1", "node-b"), pv("pv-2", "node-c")},
	}
	fs, err := (DRAIN002{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain002RequireN(t, fs, 1)[0]
	if len(f.Evidence) != 3 {
		t.Errorf("Evidence = %+v, want 3 entries (one per ordinal's PVC/PV), collapsed into a single finding", f.Evidence)
	}
}

func TestDRAIN002_LocalPV_HostnamePinned_Warning(t *testing.T) {
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain002Deployment("db", 1, []corev1.Volume{
			{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "db-data"}}},
		})},
		PersistentVolumeClaims: []corev1.PersistentVolumeClaim{{
			ObjectMeta: metav1.ObjectMeta{Name: "db-data", Namespace: "default"},
			Spec:       corev1.PersistentVolumeClaimSpec{VolumeName: "pv-local-1"},
		}},
		PersistentVolumes: []corev1.PersistentVolume{{
			ObjectMeta: metav1.ObjectMeta{Name: "pv-local-1"},
			Spec: corev1.PersistentVolumeSpec{
				PersistentVolumeSource: corev1.PersistentVolumeSource{Local: &corev1.LocalVolumeSource{Path: "/mnt/data"}},
				NodeAffinity: &corev1.VolumeNodeAffinity{Required: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{MatchExpressions: []corev1.NodeSelectorRequirement{{Key: "kubernetes.io/hostname", Operator: corev1.NodeSelectorOpIn, Values: []string{"node-b"}}}},
				}}},
			},
		}},
		Nodes: []corev1.Node{drain002ReadyNode("node-b")},
	}
	fs, err := (DRAIN002{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain002RequireN(t, fs, 1)[0]
	if f.Severity != findings.SeverityWarning {
		t.Errorf("Severity = %q, want Warning", f.Severity)
	}
}

func TestDRAIN002_LocalPV_SingletonOnNotReadyNode_Blocker(t *testing.T) {
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain002Deployment("db", 1, []corev1.Volume{
			{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "db-data"}}},
		})},
		PersistentVolumeClaims: []corev1.PersistentVolumeClaim{{
			ObjectMeta: metav1.ObjectMeta{Name: "db-data", Namespace: "default"},
			Spec:       corev1.PersistentVolumeClaimSpec{VolumeName: "pv-local-1"},
		}},
		PersistentVolumes: []corev1.PersistentVolume{{
			ObjectMeta: metav1.ObjectMeta{Name: "pv-local-1"},
			Spec: corev1.PersistentVolumeSpec{
				PersistentVolumeSource: corev1.PersistentVolumeSource{Local: &corev1.LocalVolumeSource{Path: "/mnt/data"}},
				NodeAffinity: &corev1.VolumeNodeAffinity{Required: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{MatchExpressions: []corev1.NodeSelectorRequirement{{Key: "kubernetes.io/hostname", Operator: corev1.NodeSelectorOpIn, Values: []string{"node-b"}}}},
				}}},
			},
		}},
		Nodes: []corev1.Node{drain002NotReadyNode("node-b")},
	}
	fs, err := (DRAIN002{}).Evaluate(&ScanContext{K8s: snap, UpgradeContext: findings.UpgradeContextWorkerRollout}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain002RequireN(t, fs, 1)[0]
	if f.Severity != findings.SeverityBlocker {
		t.Errorf("Severity = %q, want Blocker -- pinned node is currently NotReady", f.Severity)
	}
}

func TestDRAIN002_NetworkAttachedCSI_ZoneTopology_NoFinding(t *testing.T) {
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain002Deployment("app", 1, []corev1.Volume{
			{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "app-data"}}},
		})},
		PersistentVolumeClaims: []corev1.PersistentVolumeClaim{{
			ObjectMeta: metav1.ObjectMeta{Name: "app-data", Namespace: "default"},
			Spec:       corev1.PersistentVolumeClaimSpec{VolumeName: "pv-ebs-1"},
		}},
		PersistentVolumes: []corev1.PersistentVolume{{
			ObjectMeta: metav1.ObjectMeta{Name: "pv-ebs-1"},
			Spec: corev1.PersistentVolumeSpec{
				NodeAffinity: &corev1.VolumeNodeAffinity{Required: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{MatchExpressions: []corev1.NodeSelectorRequirement{{Key: "topology.kubernetes.io/zone", Operator: corev1.NodeSelectorOpIn, Values: []string{"us-east-1a"}}}},
				}}},
			},
		}},
	}
	fs, err := (DRAIN002{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings -- AZ-level topology affinity is not \"pinned to one node\"", fs, err)
	}
}

func TestDRAIN002_MultipleHostnameValues_NoFinding(t *testing.T) {
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain002Deployment("app", 1, []corev1.Volume{
			{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "app-data"}}},
		})},
		PersistentVolumeClaims: []corev1.PersistentVolumeClaim{{
			ObjectMeta: metav1.ObjectMeta{Name: "app-data", Namespace: "default"},
			Spec:       corev1.PersistentVolumeClaimSpec{VolumeName: "pv-multi"},
		}},
		PersistentVolumes: []corev1.PersistentVolume{{
			ObjectMeta: metav1.ObjectMeta{Name: "pv-multi"},
			Spec: corev1.PersistentVolumeSpec{
				NodeAffinity: &corev1.VolumeNodeAffinity{Required: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{MatchExpressions: []corev1.NodeSelectorRequirement{{Key: "kubernetes.io/hostname", Operator: corev1.NodeSelectorOpIn, Values: []string{"node-a", "node-b"}}}},
				}}},
			},
		}},
	}
	fs, err := (DRAIN002{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings -- multiple hostname values isn't a single-node pin", fs, err)
	}
}

func TestDRAIN002_EmptyDirOnly_NoFinding(t *testing.T) {
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain002Deployment("app", 1, []corev1.Volume{
			{Name: "scratch", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
		})},
	}
	fs, err := (DRAIN002{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings for emptyDir -- out of scope for this rule", fs, err)
	}
}

func TestDRAIN002_DaemonSetsNeverEvaluated(t *testing.T) {
	snap := &k8s.Snapshot{DaemonSets: []appsv1.DaemonSet{{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "default"},
		Spec: appsv1.DaemonSetSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{{Name: "hostlogs", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/log"}}}},
		}}},
	}}}
	fs, err := (DRAIN002{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want DaemonSets never evaluated -- hostPath-per-node is the intended pattern there", fs, err)
	}
}

func TestDRAIN002_UnboundPVC_NoFindingNoPanic(t *testing.T) {
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain002Deployment("app", 1, []corev1.Volume{
			{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "app-data"}}},
		})},
		PersistentVolumeClaims: []corev1.PersistentVolumeClaim{{
			ObjectMeta: metav1.ObjectMeta{Name: "app-data", Namespace: "default"},
			// VolumeName unset: not yet bound.
		}},
	}
	fs, err := (DRAIN002{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings for an unbound PVC", fs, err)
	}
}

func TestDRAIN002_PVNotInSnapshot_NoFindingNoPanic(t *testing.T) {
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain002Deployment("app", 1, []corev1.Volume{
			{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "app-data"}}},
		})},
		PersistentVolumeClaims: []corev1.PersistentVolumeClaim{{
			ObjectMeta: metav1.ObjectMeta{Name: "app-data", Namespace: "default"},
			Spec:       corev1.PersistentVolumeClaimSpec{VolumeName: "pv-does-not-exist"},
		}},
	}
	fs, err := (DRAIN002{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings when the referenced PV isn't in the snapshot", fs, err)
	}
}

func TestDRAIN002_MultiplePods_OneFindingPerController(t *testing.T) {
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain002Deployment("logger", 3, []corev1.Volume{
			{Name: "hostlogs", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/log"}}},
		})},
		Pods: []corev1.Pod{
			drain002Pod("logger-abc123-1", "node-a", map[string]string{"app": "logger"}),
			drain002Pod("logger-abc123-2", "node-b", map[string]string{"app": "logger"}),
			drain002Pod("logger-abc123-3", "node-c", map[string]string{"app": "logger"}),
		},
	}
	fs, err := (DRAIN002{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	drain002RequireN(t, fs, 1)
}

func TestDRAIN002_NoVolumesOfInterest_NoFinding(t *testing.T) {
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain002Deployment("app", 1, nil)},
	}
	fs, err := (DRAIN002{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings for a workload with no volumes", fs, err)
	}
}

func TestDRAIN002_NilK8sSnapshot_NoPanic(t *testing.T) {
	fs, err := (DRAIN002{}).Evaluate(&ScanContext{}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings and no panic with a nil K8s snapshot", fs, err)
	}
}
