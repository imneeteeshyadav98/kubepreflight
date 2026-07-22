package rules

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/k8s"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

func drain005RequireOne(t *testing.T, fs []findings.Finding) findings.Finding {
	t.Helper()
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
	if fs[0].RuleID != "DRAIN-005" {
		t.Errorf("RuleID = %q, want DRAIN-005", fs[0].RuleID)
	}
	return fs[0]
}

func drain005StatefulSet(name string, replicas, ready int32) appsv1.StatefulSet {
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", UID: types.UID("uid-" + name)},
		Spec:       appsv1.StatefulSetSpec{Replicas: int32Ptr(replicas)},
		Status:     appsv1.StatefulSetStatus{Replicas: replicas, ReadyReplicas: ready},
	}
}

func drain005DaemonSet(name string, desired, ready int32) appsv1.DaemonSet {
	return appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", UID: types.UID("uid-" + name)},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: desired,
			NumberReady:            ready,
			NumberAvailable:        ready,
			NumberUnavailable:      desired - ready,
		},
	}
}

func TestDRAIN005_StatefulSet_AllReady_NoFinding(t *testing.T) {
	snap := &k8s.Snapshot{StatefulSets: []appsv1.StatefulSet{drain005StatefulSet("db", 3, 3)}}
	fs, err := (DRAIN005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings when fully Ready", fs, err)
	}
}

func TestDRAIN005_StatefulSet_PartiallyReady_Warning(t *testing.T) {
	snap := &k8s.Snapshot{StatefulSets: []appsv1.StatefulSet{drain005StatefulSet("db", 3, 2)}}
	fs, err := (DRAIN005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain005RequireOne(t, fs)
	if f.Severity != findings.SeverityWarning {
		t.Errorf("Severity = %q, want Warning", f.Severity)
	}
	if f.Resources[0].Kind != "StatefulSet" {
		t.Errorf("Resources[0].Kind = %q, want StatefulSet", f.Resources[0].Kind)
	}
}

func TestDRAIN005_StatefulSet_ZeroReady_OrdinaryWorkloadWarning(t *testing.T) {
	snap := &k8s.Snapshot{StatefulSets: []appsv1.StatefulSet{drain005StatefulSet("db", 3, 0)}}
	fs, err := (DRAIN005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain005RequireOne(t, fs)
	if f.Severity != findings.SeverityWarning || f.UpgradeGate != findings.UpgradeGateOperatorDecision {
		t.Errorf("Severity/Gate = %q/%q, want Warning/operator_decision for ordinary zero-ready workload", f.Severity, f.UpgradeGate)
	}
}

func TestDRAIN005_StatefulSet_ZeroReadyCriticalInfraWorkerRollout_Blocker(t *testing.T) {
	snap := &k8s.Snapshot{StatefulSets: []appsv1.StatefulSet{drain005StatefulSet("coredns", 3, 0)}}
	fs, err := (DRAIN005{}).Evaluate(&ScanContext{K8s: snap, UpgradeContext: findings.UpgradeContextWorkerRollout}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain005RequireOne(t, fs)
	if f.Severity != findings.SeverityBlocker || f.UpgradeGate != findings.UpgradeGateBlock {
		t.Errorf("Severity/Gate = %q/%q, want Blocker/block", f.Severity, f.UpgradeGate)
	}
}

func TestDRAIN005_StatefulSet_ZeroDesiredReplicas_NoFinding(t *testing.T) {
	snap := &k8s.Snapshot{StatefulSets: []appsv1.StatefulSet{drain005StatefulSet("db", 0, 0)}}
	fs, err := (DRAIN005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings for a scaled-to-zero StatefulSet", fs, err)
	}
}

func TestDRAIN005_StatefulSet_DeletingController_NoFinding(t *testing.T) {
	now := metav1.Now()
	sts := drain005StatefulSet("db", 3, 1)
	sts.DeletionTimestamp = &now
	snap := &k8s.Snapshot{StatefulSets: []appsv1.StatefulSet{sts}}
	fs, err := (DRAIN005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings for a StatefulSet being deleted", fs, err)
	}
}

func TestDRAIN005_StatefulSet_PartitionEvidence(t *testing.T) {
	sts := drain005StatefulSet("db", 3, 2)
	sts.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{
		Type:          appsv1.RollingUpdateStatefulSetStrategyType,
		RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{Partition: int32Ptr(1)},
	}
	snap := &k8s.Snapshot{StatefulSets: []appsv1.StatefulSet{sts}}
	fs, err := (DRAIN005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain005RequireOne(t, fs)
	found := false
	for _, e := range f.Evidence {
		if e == "updateStrategy: RollingUpdate (partition: 1 — ordinals below this are intentionally held back)" {
			found = true
		}
	}
	if !found {
		t.Errorf("Evidence = %+v, want partition context surfaced", f.Evidence)
	}
}

func TestDRAIN005_StatefulSet_CriticalInfraName_Escalates(t *testing.T) {
	snap := &k8s.Snapshot{StatefulSets: []appsv1.StatefulSet{drain005StatefulSet("coredns", 2, 1)}}
	fs, err := (DRAIN005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain005RequireOne(t, fs)
	if !f.CriticalInfra {
		t.Error("CriticalInfra = false, want true for a well-known critical-infra name")
	}
}

func TestDRAIN005_DaemonSet_AllReady_NoFinding(t *testing.T) {
	snap := &k8s.Snapshot{DaemonSets: []appsv1.DaemonSet{drain005DaemonSet("agent", 3, 3)}}
	fs, err := (DRAIN005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings when fully Ready", fs, err)
	}
}

func TestDRAIN005_DaemonSet_PartiallyReady_Warning(t *testing.T) {
	snap := &k8s.Snapshot{DaemonSets: []appsv1.DaemonSet{drain005DaemonSet("agent", 3, 2)}}
	fs, err := (DRAIN005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain005RequireOne(t, fs)
	if f.Severity != findings.SeverityWarning {
		t.Errorf("Severity = %q, want Warning", f.Severity)
	}
	if f.Resources[0].Kind != "DaemonSet" {
		t.Errorf("Resources[0].Kind = %q, want DaemonSet", f.Resources[0].Kind)
	}
}

func TestDRAIN005_DaemonSet_ZeroReady_OrdinaryWorkloadWarning(t *testing.T) {
	snap := &k8s.Snapshot{DaemonSets: []appsv1.DaemonSet{drain005DaemonSet("agent", 3, 0)}}
	fs, err := (DRAIN005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain005RequireOne(t, fs)
	if f.Severity != findings.SeverityWarning || f.UpgradeGate != findings.UpgradeGateOperatorDecision {
		t.Errorf("Severity/Gate = %q/%q, want Warning/operator_decision", f.Severity, f.UpgradeGate)
	}
}

func TestDRAIN005_DaemonSet_ZeroReadyCriticalInfraWorkerRollout_Blocker(t *testing.T) {
	snap := &k8s.Snapshot{DaemonSets: []appsv1.DaemonSet{drain005DaemonSet("kube-proxy", 3, 0)}}
	fs, err := (DRAIN005{}).Evaluate(&ScanContext{K8s: snap, UpgradeContext: findings.UpgradeContextWorkerRollout}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain005RequireOne(t, fs)
	if f.Severity != findings.SeverityBlocker || f.UpgradeGate != findings.UpgradeGateBlock {
		t.Errorf("Severity/Gate = %q/%q, want Blocker/block", f.Severity, f.UpgradeGate)
	}
}

func TestDRAIN005_DaemonSet_ZeroDesired_NoFinding(t *testing.T) {
	snap := &k8s.Snapshot{DaemonSets: []appsv1.DaemonSet{drain005DaemonSet("agent", 0, 0)}}
	fs, err := (DRAIN005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings when no nodes are targeted", fs, err)
	}
}

func TestDRAIN005_DaemonSet_CriticalInfraName_Escalates(t *testing.T) {
	snap := &k8s.Snapshot{DaemonSets: []appsv1.DaemonSet{drain005DaemonSet("kube-proxy", 3, 2)}}
	fs, err := (DRAIN005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain005RequireOne(t, fs)
	if !f.CriticalInfra {
		t.Error("CriticalInfra = false, want true for kube-proxy")
	}
	if f.Severity != findings.SeverityWarning {
		t.Errorf("Severity = %q, want Warning (CriticalInfra escalates priority, not severity)", f.Severity)
	}
}

func TestDRAIN005_DaemonSet_NonCriticalName_NoEscalation(t *testing.T) {
	snap := &k8s.Snapshot{DaemonSets: []appsv1.DaemonSet{drain005DaemonSet("log-shipper", 3, 2)}}
	fs, err := (DRAIN005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain005RequireOne(t, fs)
	if f.CriticalInfra {
		t.Error("CriticalInfra = true, want false for an arbitrary DaemonSet name")
	}
}

func TestDRAIN005_DaemonSet_MaxUnavailableEvidence(t *testing.T) {
	ds := drain005DaemonSet("agent", 3, 2)
	mu := intstr.FromInt(2)
	ds.Spec.UpdateStrategy = appsv1.DaemonSetUpdateStrategy{
		Type:          appsv1.RollingUpdateDaemonSetStrategyType,
		RollingUpdate: &appsv1.RollingUpdateDaemonSet{MaxUnavailable: &mu},
	}
	snap := &k8s.Snapshot{DaemonSets: []appsv1.DaemonSet{ds}}
	fs, err := (DRAIN005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := drain005RequireOne(t, fs)
	found := false
	for _, e := range f.Evidence {
		if e == "updateStrategy: RollingUpdate (maxUnavailable: 2)" {
			found = true
		}
	}
	if !found {
		t.Errorf("Evidence = %+v, want maxUnavailable surfaced", f.Evidence)
	}
}

func TestDRAIN005_DaemonSet_DeletingController_NoFinding(t *testing.T) {
	now := metav1.Now()
	ds := drain005DaemonSet("agent", 3, 1)
	ds.DeletionTimestamp = &now
	snap := &k8s.Snapshot{DaemonSets: []appsv1.DaemonSet{ds}}
	fs, err := (DRAIN005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings for a DaemonSet being deleted", fs, err)
	}
}

func TestDRAIN005_NilK8sSnapshot_NoPanic(t *testing.T) {
	fs, err := (DRAIN005{}).Evaluate(&ScanContext{}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings and no panic", fs, err)
	}
}
