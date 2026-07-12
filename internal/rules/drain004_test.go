package rules

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"kubepreflight/internal/collectors/k8s"
	"kubepreflight/internal/findings"
)

func drain004Node(name, cpu, mem string) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, UID: types.UID("uid-" + name)},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(cpu), corev1.ResourceMemory: resource.MustParse(mem)},
			Conditions:  []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
		},
	}
}

func drain004Pod(name, nodeName, cpu, mem string, owner *metav1.OwnerReference) corev1.Pod {
	p := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{{
				Name: "app",
				Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse(cpu), corev1.ResourceMemory: resource.MustParse(mem),
				}},
			}},
		},
	}
	if owner != nil {
		p.OwnerReferences = []metav1.OwnerReference{*owner}
	}
	return p
}

func drain004RequireOne(t *testing.T, fs []findings.Finding) findings.Finding {
	t.Helper()
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
	if fs[0].RuleID != "DRAIN-004" {
		t.Errorf("RuleID = %q, want DRAIN-004", fs[0].RuleID)
	}
	return fs[0]
}

func TestDRAIN004_SingleNodeCluster_NoFinding(t *testing.T) {
	snap := &k8s.Snapshot{Nodes: []corev1.Node{drain004Node("node-a", "1", "1Gi")}}
	fs, err := (DRAIN004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings for a single-node cluster", fs, err)
	}
}

func TestDRAIN004_HealthyCapacity_NoFinding(t *testing.T) {
	snap := &k8s.Snapshot{
		Nodes: []corev1.Node{drain004Node("node-a", "4", "8Gi"), drain004Node("node-b", "4", "8Gi")},
		Pods: []corev1.Pod{
			drain004Pod("app-1", "node-a", "500m", "512Mi", nil),
		},
	}
	fs, err := (DRAIN004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings with plenty of spare capacity", fs, err)
	}
}

func TestDRAIN004_CPUShortage_Warning(t *testing.T) {
	snap := &k8s.Snapshot{
		Nodes: []corev1.Node{drain004Node("node-a", "4", "16Gi"), drain004Node("node-b", "1", "16Gi")},
		Pods: []corev1.Pod{
			// node-a hosts 3.5 CPU worth of pods; node-b only has 1 CPU allocatable total.
			drain004Pod("app-1", "node-a", "3500m", "1Gi", nil),
		},
	}
	fs, err := (DRAIN004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain004RequireOne(t, fs)
	if f.Severity != findings.SeverityWarning {
		t.Errorf("Severity = %q, want Warning (never Blocker, per design)", f.Severity)
	}
	if f.Confidence != findings.TierInferred {
		t.Errorf("Confidence = %q, want INFERRED", f.Confidence)
	}
	if f.Resources[0].Name != "node-a" {
		t.Errorf("Resources[0].Name = %q, want node-a (the node under evaluation)", f.Resources[0].Name)
	}
}

func TestDRAIN004_MemoryShortage_Warning(t *testing.T) {
	snap := &k8s.Snapshot{
		Nodes: []corev1.Node{drain004Node("node-a", "8", "16Gi"), drain004Node("node-b", "8", "1Gi")},
		Pods: []corev1.Pod{
			drain004Pod("app-1", "node-a", "1", "15Gi", nil),
		},
	}
	fs, err := (DRAIN004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	drain004RequireOne(t, fs)
}

func TestDRAIN004_DaemonSetPodsExcludedFromDisplacedDemand(t *testing.T) {
	dsOwner := &metav1.OwnerReference{Kind: "DaemonSet", Name: "agent"}
	snap := &k8s.Snapshot{
		Nodes: []corev1.Node{drain004Node("node-a", "1", "1Gi"), drain004Node("node-b", "1", "1Gi")},
		Pods: []corev1.Pod{
			// Entirely DaemonSet-owned: shouldn't count as displaced demand at all.
			drain004Pod("agent-a", "node-a", "900m", "900Mi", dsOwner),
		},
	}
	fs, err := (DRAIN004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings -- DaemonSet pods aren't displaced demand", fs, err)
	}
}

func TestDRAIN004_DaemonSetPodsCountTowardRemainingNodeUsage(t *testing.T) {
	dsOwner := &metav1.OwnerReference{Kind: "DaemonSet", Name: "agent"}
	snap := &k8s.Snapshot{
		Nodes: []corev1.Node{drain004Node("node-a", "1", "1Gi"), drain004Node("node-b", "1", "1Gi")},
		Pods: []corev1.Pod{
			drain004Pod("app-1", "node-a", "800m", "100Mi", nil),
			// node-b already has a DaemonSet pod consuming most of its capacity.
			drain004Pod("agent-b", "node-b", "900m", "100Mi", dsOwner),
		},
	}
	fs, err := (DRAIN004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	drain004RequireOne(t, fs)
}

func TestDRAIN004_MissingRequests_ExcludedNotZeroCost(t *testing.T) {
	noRequests := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "no-requests", Namespace: "default"},
		Spec:       corev1.PodSpec{NodeName: "node-a", Containers: []corev1.Container{{Name: "app"}}},
	}
	snap := &k8s.Snapshot{
		Nodes: []corev1.Node{drain004Node("node-a", "1", "1Gi"), drain004Node("node-b", "1", "1Gi")},
		Pods:  []corev1.Pod{noRequests},
	}
	fs, err := (DRAIN004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings -- a request-less pod must not be assumed zero-cost into a shortage claim, nor should it alone trigger one", fs, err)
	}
}

func TestDRAIN004_MissingRequests_SurfacedAsCoverageCaveat(t *testing.T) {
	noRequests := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "no-requests", Namespace: "default"},
		Spec:       corev1.PodSpec{NodeName: "node-a", Containers: []corev1.Container{{Name: "app"}}},
	}
	snap := &k8s.Snapshot{
		Nodes: []corev1.Node{drain004Node("node-a", "4", "16Gi"), drain004Node("node-b", "1", "1Gi")},
		Pods: []corev1.Pod{
			drain004Pod("app-1", "node-a", "3500m", "1Gi", nil),
			noRequests,
		},
	}
	fs, err := (DRAIN004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain004RequireOne(t, fs)
	found := false
	for _, e := range f.Evidence {
		if e == "pods excluded from this estimate due to incomplete resource requests: 1" {
			found = true
		}
	}
	if !found {
		t.Errorf("Evidence = %+v, want the incomplete-requests caveat surfaced", f.Evidence)
	}
}

func TestDRAIN004_PendingPodsAddToDemand(t *testing.T) {
	pending := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pending-app", Namespace: "default"},
		Status:     corev1.PodStatus{Phase: corev1.PodPending},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name: "app",
			Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("900m"), corev1.ResourceMemory: resource.MustParse("100Mi"),
			}},
		}}},
	}
	snap := &k8s.Snapshot{
		Nodes: []corev1.Node{drain004Node("node-a", "1", "1Gi"), drain004Node("node-b", "1", "1Gi")},
		Pods: []corev1.Pod{
			drain004Pod("app-1", "node-a", "500m", "100Mi", nil),
			pending,
		},
	}
	fs, err := (DRAIN004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	// node-a's own displaced demand (500m) alone fits in node-b's 1 CPU, but
	// the pending pod's 900m competing for the same spare capacity tips it
	// into shortage.
	drain004RequireOne(t, fs)
}

// TestDRAIN004_NotReadyNodeContributesNoCapacity confirms a NotReady node
// is excluded both as a removal candidate and as a spare-capacity source
// -- with node-b broken, node-a is genuinely the only usable node, so
// removing it really would leave its pods with nowhere to go. That's a
// real risk, not a false positive: the check correctly fires here.
func TestDRAIN004_NotReadyNodeContributesNoCapacity(t *testing.T) {
	notReady := drain004Node("node-b", "4", "8Gi")
	notReady.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse}}
	snap := &k8s.Snapshot{
		Nodes: []corev1.Node{drain004Node("node-a", "1", "1Gi"), notReady},
		Pods: []corev1.Pod{
			drain004Pod("app-1", "node-a", "900m", "100Mi", nil),
		},
	}
	fs, err := (DRAIN004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain004RequireOne(t, fs)
	if f.Resources[0].Name != "node-a" {
		t.Errorf("Resources[0].Name = %q, want node-a", f.Resources[0].Name)
	}
	for _, e := range f.Evidence {
		if e == "estimated spare capacity on remaining nodes: CPU 0, memory 0" {
			return
		}
	}
	t.Errorf("Evidence = %+v, want zero spare capacity (node-b is NotReady, contributes nothing)", f.Evidence)
}

// TestDRAIN004_CordonedNodeContributesNoCapacity is the same scenario via
// Unschedulable instead of NotReady -- a cordoned node also shouldn't be
// counted as spare capacity.
func TestDRAIN004_CordonedNodeContributesNoCapacity(t *testing.T) {
	cordoned := drain004Node("node-b", "4", "8Gi")
	cordoned.Spec.Unschedulable = true
	snap := &k8s.Snapshot{
		Nodes: []corev1.Node{drain004Node("node-a", "1", "1Gi"), cordoned},
		Pods: []corev1.Pod{
			drain004Pod("app-1", "node-a", "900m", "100Mi", nil),
		},
	}
	fs, err := (DRAIN004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	drain004RequireOne(t, fs)
}

// TestDRAIN004_ControlPlaneTaintExcludedFromCapacity guards a real bug
// found via kind-cluster testing: the control-plane node's full
// allocatable capacity was being counted as spare for ordinary workload
// pods, even though its NoSchedule taint means they can never actually
// land there. Without this exclusion, the estimate silently understates
// risk on every real cluster (every cluster has a control-plane node).
func TestDRAIN004_ControlPlaneTaintExcludedFromCapacity(t *testing.T) {
	controlPlane := drain004Node("control-plane", "20", "16Gi")
	controlPlane.Spec.Taints = []corev1.Taint{{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule}}
	snap := &k8s.Snapshot{
		Nodes: []corev1.Node{controlPlane, drain004Node("worker", "1", "1Gi")},
		Pods: []corev1.Pod{
			drain004Pod("app-1", "worker", "900m", "100Mi", nil),
		},
	}
	fs, err := (DRAIN004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain004RequireOne(t, fs)
	for _, e := range f.Evidence {
		if e == "estimated spare capacity on remaining nodes: CPU 0, memory 0" {
			return
		}
	}
	t.Errorf("Evidence = %+v, want zero spare capacity -- the control-plane's 20 CPU/16Gi must not be counted", f.Evidence)
}

func TestDRAIN004_NilK8sSnapshot_NoPanic(t *testing.T) {
	fs, err := (DRAIN004{}).Evaluate(&ScanContext{}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings and no panic", fs, err)
	}
}
