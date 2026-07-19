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

func drain003Node(name string, labels map[string]string, taints []corev1.Taint) corev1.Node {
	return corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}, Spec: corev1.NodeSpec{Taints: taints}}
}

func drain003Deployment(name string, replicas int32, podSpec corev1.PodSpec) appsv1.Deployment {
	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", UID: types.UID("uid-" + name)},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(replicas),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": name}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": name}},
				Spec:       podSpec,
			},
		},
	}
}

func drain003RequireDiscriminator(t *testing.T, fs []findings.Finding, want string) findings.Finding {
	t.Helper()
	for _, f := range fs {
		if f.RuleID != "DRAIN-003" {
			t.Errorf("RuleID = %q, want DRAIN-003", f.RuleID)
		}
	}
	t.Logf("got %d findings", len(fs))
	if len(fs) == 0 {
		t.Fatalf("got 0 findings, want one containing discriminator %q", want)
	}
	return fs[0]
}

func TestDRAIN003_NoConstraints_NoFinding(t *testing.T) {
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain003Deployment("app", 2, corev1.PodSpec{})},
		Nodes:       []corev1.Node{drain003Node("node-a", nil, nil), drain003Node("node-b", nil, nil)},
	}
	fs, err := (DRAIN003{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings for a plain pod template", fs, err)
	}
}

func TestDRAIN003_NodeSelectorScarcity_OneQualifyingNode_Warning(t *testing.T) {
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain003Deployment("gpu-app", 1, corev1.PodSpec{
			NodeSelector: map[string]string{"gpu": "true"},
		})},
		Nodes: []corev1.Node{
			drain003Node("gpu-node-1", map[string]string{"gpu": "true"}, nil),
			drain003Node("plain-node", nil, nil),
		},
	}
	fs, err := (DRAIN003{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain003RequireDiscriminator(t, fs, "scarcity")
	if f.Severity != findings.SeverityWarning {
		t.Errorf("Severity = %q, want Warning", f.Severity)
	}
}

func TestDRAIN003_NodeSelectorScarcity_MultipleQualifyingNodes_NoFinding(t *testing.T) {
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain003Deployment("gpu-app", 1, corev1.PodSpec{
			NodeSelector: map[string]string{"gpu": "true"},
		})},
		Nodes: []corev1.Node{
			drain003Node("gpu-node-1", map[string]string{"gpu": "true"}, nil),
			drain003Node("gpu-node-2", map[string]string{"gpu": "true"}, nil),
		},
	}
	fs, err := (DRAIN003{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings with 2 qualifying nodes", fs, err)
	}
}

func TestDRAIN003_RequiredNodeAffinity_Scarcity_Warning(t *testing.T) {
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain003Deployment("special-app", 1, corev1.PodSpec{
			Affinity: &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{MatchExpressions: []corev1.NodeSelectorRequirement{{Key: "zone", Operator: corev1.NodeSelectorOpIn, Values: []string{"z1"}}}},
				},
			}}},
		})},
		Nodes: []corev1.Node{
			drain003Node("node-z1", map[string]string{"zone": "z1"}, nil),
			drain003Node("node-z2", map[string]string{"zone": "z2"}, nil),
		},
	}
	fs, err := (DRAIN003{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	drain003RequireDiscriminator(t, fs, "scarcity")
}

func TestDRAIN003_Toleration_ExcludesUntoleratedTaintedNode(t *testing.T) {
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain003Deployment("app", 1, corev1.PodSpec{
			NodeSelector: map[string]string{"pool": "special"},
		})},
		Nodes: []corev1.Node{
			drain003Node("node-a", map[string]string{"pool": "special"}, []corev1.Taint{{Key: "dedicated", Value: "special", Effect: corev1.TaintEffectNoSchedule}}),
			drain003Node("node-b", map[string]string{"pool": "special"}, nil),
		},
	}
	// No toleration for the dedicated=special taint -- only node-b qualifies.
	fs, err := (DRAIN003{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain003RequireDiscriminator(t, fs, "scarcity")
	found := false
	for _, e := range f.Evidence {
		if e == "qualifying node(s): node-b" {
			found = true
		}
	}
	if !found {
		t.Errorf("Evidence = %+v, want only node-b to qualify (node-a's taint isn't tolerated)", f.Evidence)
	}
}

func TestDRAIN003_AntiAffinity_HostnameTopology_NoSpareNode_Warning(t *testing.T) {
	antiAffinity := &corev1.Affinity{PodAntiAffinity: &corev1.PodAntiAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{TopologyKey: "kubernetes.io/hostname"}},
	}}
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain003Deployment("spread-app", 2, corev1.PodSpec{Affinity: antiAffinity})},
		Nodes:       []corev1.Node{drain003Node("node-a", nil, nil), drain003Node("node-b", nil, nil)},
	}
	fs, err := (DRAIN003{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	drain003RequireDiscriminator(t, fs, "anti-affinity")
}

func TestDRAIN003_AntiAffinity_HostnameTopology_SpareNode_NoFinding(t *testing.T) {
	antiAffinity := &corev1.Affinity{PodAntiAffinity: &corev1.PodAntiAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{TopologyKey: "kubernetes.io/hostname"}},
	}}
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain003Deployment("spread-app", 2, corev1.PodSpec{Affinity: antiAffinity})},
		Nodes:       []corev1.Node{drain003Node("node-a", nil, nil), drain003Node("node-b", nil, nil), drain003Node("node-c", nil, nil)},
	}
	fs, err := (DRAIN003{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings with a spare node", fs, err)
	}
}

func TestDRAIN003_AntiAffinity_NonHostnameTopologyKey_Ignored(t *testing.T) {
	antiAffinity := &corev1.Affinity{PodAntiAffinity: &corev1.PodAntiAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{TopologyKey: "topology.kubernetes.io/zone"}},
	}}
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain003Deployment("spread-app", 5, corev1.PodSpec{Affinity: antiAffinity})},
		Nodes:       []corev1.Node{drain003Node("node-a", nil, nil), drain003Node("node-b", nil, nil)},
	}
	fs, err := (DRAIN003{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want zone-topology anti-affinity out of scope for this PR", fs, err)
	}
}

func TestDRAIN003_TopologySpread_DoNotSchedule_SingleDomain_Warning(t *testing.T) {
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain003Deployment("app", 2, corev1.PodSpec{
			TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
				{MaxSkew: 1, TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: corev1.DoNotSchedule},
			},
		})},
		Nodes: []corev1.Node{
			drain003Node("node-a", map[string]string{"topology.kubernetes.io/zone": "z1"}, nil),
			drain003Node("node-b", map[string]string{"topology.kubernetes.io/zone": "z1"}, nil),
		},
	}
	fs, err := (DRAIN003{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	drain003RequireDiscriminator(t, fs, "topology-spread")
}

func TestDRAIN003_TopologySpread_DoNotSchedule_MultipleDomains_NoFinding(t *testing.T) {
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain003Deployment("app", 2, corev1.PodSpec{
			TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
				{MaxSkew: 1, TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: corev1.DoNotSchedule},
			},
		})},
		Nodes: []corev1.Node{
			drain003Node("node-a", map[string]string{"topology.kubernetes.io/zone": "z1"}, nil),
			drain003Node("node-b", map[string]string{"topology.kubernetes.io/zone": "z2"}, nil),
		},
	}
	fs, err := (DRAIN003{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings with 2 topology domains", fs, err)
	}
}

func TestDRAIN003_TopologySpread_ScheduleAnyway_Ignored(t *testing.T) {
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain003Deployment("app", 2, corev1.PodSpec{
			TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
				{MaxSkew: 1, TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: corev1.ScheduleAnyway},
			},
		})},
		Nodes: []corev1.Node{drain003Node("node-a", map[string]string{"topology.kubernetes.io/zone": "z1"}, nil)},
	}
	fs, err := (DRAIN003{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want ScheduleAnyway constraints out of scope", fs, err)
	}
}

func TestDRAIN003_HostPort_AllOtherNodesOccupied_Warning(t *testing.T) {
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain003Deployment("ingress", 1, corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app", Ports: []corev1.ContainerPort{{HostPort: 443}}}},
		})},
		Pods: []corev1.Pod{
			{ObjectMeta: metav1.ObjectMeta{Name: "ingress-abc-1", Namespace: "default", Labels: map[string]string{"app": "ingress"}, OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "ingress-abc"}}},
				Spec: corev1.PodSpec{NodeName: "node-a", Containers: []corev1.Container{{Ports: []corev1.ContainerPort{{HostPort: 443}}}}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "other-app", Namespace: "default"},
				Spec: corev1.PodSpec{NodeName: "node-b", Containers: []corev1.Container{{Ports: []corev1.ContainerPort{{HostPort: 443}}}}}},
		},
		Nodes: []corev1.Node{drain003Node("node-a", nil, nil), drain003Node("node-b", nil, nil)},
	}
	fs, err := (DRAIN003{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain003RequireDiscriminator(t, fs, "hostport")
	if f.Severity != findings.SeverityWarning {
		t.Errorf("Severity = %q, want Warning", f.Severity)
	}
}

func TestDRAIN003_HostPort_FreeNodeAvailable_NoFinding(t *testing.T) {
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain003Deployment("ingress", 1, corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app", Ports: []corev1.ContainerPort{{HostPort: 443}}}},
		})},
		Pods: []corev1.Pod{
			{ObjectMeta: metav1.ObjectMeta{Name: "ingress-abc-1", Namespace: "default", Labels: map[string]string{"app": "ingress"}, OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "ingress-abc"}}},
				Spec: corev1.PodSpec{NodeName: "node-a", Containers: []corev1.Container{{Ports: []corev1.ContainerPort{{HostPort: 443}}}}}},
		},
		Nodes: []corev1.Node{drain003Node("node-a", nil, nil), drain003Node("node-b", nil, nil)},
	}
	fs, err := (DRAIN003{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings -- node-b is free", fs, err)
	}
}

func TestDRAIN003_NoHostPort_NoFinding(t *testing.T) {
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain003Deployment("app", 1, corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app", Ports: []corev1.ContainerPort{{ContainerPort: 8080}}}},
		})},
		Nodes: []corev1.Node{drain003Node("node-a", nil, nil)},
	}
	fs, err := (DRAIN003{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings for a plain containerPort (no hostPort)", fs, err)
	}
}

func TestDRAIN003_DaemonSetsNeverEvaluated(t *testing.T) {
	snap := &k8s.Snapshot{
		DaemonSets: []appsv1.DaemonSet{{
			ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "default"},
			Spec: appsv1.DaemonSetSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
				NodeSelector: map[string]string{"pool": "rare"},
			}}},
		}},
		Nodes: []corev1.Node{drain003Node("node-a", map[string]string{"pool": "rare"}, nil)},
	}
	fs, err := (DRAIN003{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want DaemonSets never evaluated by DRAIN-003", fs, err)
	}
}

func TestDRAIN003_DeletingController_NoFinding(t *testing.T) {
	now := metav1.Now()
	d := drain003Deployment("app", 1, corev1.PodSpec{NodeSelector: map[string]string{"pool": "rare"}})
	d.DeletionTimestamp = &now
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{d},
		Nodes:       []corev1.Node{drain003Node("node-a", map[string]string{"pool": "rare"}, nil)},
	}
	fs, err := (DRAIN003{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings for a Deployment being deleted", fs, err)
	}
}

func TestDRAIN003_NilK8sSnapshot_NoPanic(t *testing.T) {
	fs, err := (DRAIN003{}).Evaluate(&ScanContext{}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings and no panic", fs, err)
	}
}
