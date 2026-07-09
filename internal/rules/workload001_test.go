package rules

import (
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"kubepreflight/internal/collectors/k8s"
	"kubepreflight/internal/findings"
)

func TestWORKLOAD001_ImagePullBackOffCreatesFinding(t *testing.T) {
	fs, err := (WORKLOAD001{}).Evaluate(&ScanContext{K8s: &k8s.Snapshot{Pods: []corev1.Pod{
		podWithWaitingContainer("kp-demo", "unhealthy-image-app-abc", "app", corev1.PodPending, "ImagePullBackOff", false, 0),
	}}}, "1.33")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("findings = %d, want 1: %+v", len(fs), fs)
	}
	f := fs[0]
	if f.RuleID != "WORKLOAD-001" || f.Severity != findings.SeverityWarning || f.Confidence != findings.TierObserved {
		t.Fatalf("finding identity = %+v, want WORKLOAD-001 warning observed", f)
	}
	if f.Resources[0].Kind != "Pod" || f.Resources[0].Namespace != "kp-demo" || f.Resources[0].Name != "unhealthy-image-app-abc" {
		t.Fatalf("resource = %+v, want pod reference", f.Resources[0])
	}
	for _, want := range []string{"namespace: kp-demo", "pod: unhealthy-image-app-abc", "phase: Pending", "container: app", "reason: ImagePullBackOff", "ready: false", "restartCount: 0"} {
		if !contains(f.Evidence, want) {
			t.Errorf("evidence missing %q: %+v", want, f.Evidence)
		}
	}
	if !strings.Contains(f.Message, "1 pod in ImagePullBackOff") {
		t.Errorf("message = %q, want ImagePullBackOff summary", f.Message)
	}
	if f.RemediationDetail == nil || f.RemediationDetail.SafeFix == nil || !strings.Contains(f.RemediationDetail.SafeFix.Command, "kubectl describe pod 'unhealthy-image-app-abc' -n 'kp-demo'") {
		t.Fatalf("remediation detail missing inspect commands: %+v", f.RemediationDetail)
	}

	r := findings.NewReport("1.33", "kind", "", time.Now(), fs)
	if !r.Findings[0].CanUpgradeContinue || r.Findings[0].Priority != "P4" || r.Findings[0].AffectedScope != "workload" {
		t.Fatalf("report-enriched WORKLOAD-001 = %+v, want P4 workload and can continue for warning", r.Findings[0])
	}
}

func TestWORKLOAD001_CrashLoopBackOffCreatesFinding(t *testing.T) {
	fs, err := (WORKLOAD001{}).Evaluate(&ScanContext{K8s: &k8s.Snapshot{Pods: []corev1.Pod{
		podWithWaitingContainer("default", "api-abc", "api", corev1.PodRunning, "CrashLoopBackOff", false, 7),
	}}}, "1.33")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 || !contains(fs[0].Evidence, "reason: CrashLoopBackOff") || !contains(fs[0].Evidence, "restartCount: 7") {
		t.Fatalf("findings = %+v, want CrashLoopBackOff with restart count", fs)
	}
}

func TestWORKLOAD001_HealthyRunningPodDoesNotCreateFinding(t *testing.T) {
	fs, err := (WORKLOAD001{}).Evaluate(&ScanContext{K8s: &k8s.Snapshot{Pods: []corev1.Pod{
		healthyRunningPod("default", "api-abc"),
	}}}, "1.33")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("findings = %+v, want none", fs)
	}
}

func TestWORKLOAD001_CompletedPodDoesNotCreateFinding(t *testing.T) {
	fs, err := (WORKLOAD001{}).Evaluate(&ScanContext{K8s: &k8s.Snapshot{Pods: []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "job-done", Namespace: "default", UID: "uid-job-done"},
			Status:     corev1.PodStatus{Phase: corev1.PodSucceeded},
		},
	}}}, "1.33")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("findings = %+v, want none", fs)
	}
}

func TestWORKLOAD001_DeletingPodDoesNotCreateFinding(t *testing.T) {
	deletingAt := metav1.NewTime(time.Now())
	pod := podWithWaitingContainer("default", "terminating", "app", corev1.PodPending, "ImagePullBackOff", false, 0)
	pod.DeletionTimestamp = &deletingAt
	fs, err := (WORKLOAD001{}).Evaluate(&ScanContext{K8s: &k8s.Snapshot{Pods: []corev1.Pod{pod}}}, "1.33")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("findings = %+v, want none", fs)
	}
}

func TestWORKLOAD001_MultipleUnhealthyPodsAreDeterministic(t *testing.T) {
	fs, err := (WORKLOAD001{}).Evaluate(&ScanContext{K8s: &k8s.Snapshot{Pods: []corev1.Pod{
		podWithWaitingContainer("b", "z-api", "app", corev1.PodPending, "ImagePullBackOff", false, 0),
		podWithWaitingContainer("a", "b-api", "app", corev1.PodRunning, "CrashLoopBackOff", false, 3),
		podWithWaitingContainer("a", "a-api", "app", corev1.PodPending, "ErrImagePull", false, 0),
	}}}, "1.33")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 3 {
		t.Fatalf("findings = %d, want 3: %+v", len(fs), fs)
	}
	got := []string{
		fs[0].Resources[0].Namespace + "/" + fs[0].Resources[0].Name,
		fs[1].Resources[0].Namespace + "/" + fs[1].Resources[0].Name,
		fs[2].Resources[0].Namespace + "/" + fs[2].Resources[0].Name,
	}
	want := []string{"a/a-api", "a/b-api", "b/z-api"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %+v, want %+v", got, want)
		}
	}
}

func podWithWaitingContainer(namespace, name, container string, phase corev1.PodPhase, reason string, ready bool, restarts int32) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, UID: types.UID("uid-" + name)},
		Status: corev1.PodStatus{
			Phase: phase,
			Conditions: []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: conditionStatus(ready),
			}},
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:         container,
				Ready:        ready,
				RestartCount: restarts,
				State:        corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: reason}},
			}},
		},
	}
}

func healthyRunningPod(namespace, name string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, UID: types.UID("uid-" + name)},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			}},
			ContainerStatuses: []corev1.ContainerStatus{{Name: "app", Ready: true}},
		},
	}
}

func conditionStatus(ready bool) corev1.ConditionStatus {
	if ready {
		return corev1.ConditionTrue
	}
	return corev1.ConditionFalse
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
