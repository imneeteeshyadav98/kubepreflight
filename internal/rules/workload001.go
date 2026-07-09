package rules

import (
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	"kubepreflight/internal/findings"
)

const workloadNotReadyGracePeriod = 5 * time.Minute

var workloadUnhealthyWaitingReasons = map[string]struct{}{
	"ImagePullBackOff":           {},
	"ErrImagePull":               {},
	"CrashLoopBackOff":           {},
	"CreateContainerConfigError": {},
	"CreateContainerError":       {},
	"RunContainerError":          {},
}

// WORKLOAD001 flags pods that are already unhealthy before an upgrade. This
// keeps pre-existing application breakage visible so it is not mistaken for a
// post-upgrade regression during validation.
type WORKLOAD001 struct{}

func (WORKLOAD001) ID() string { return "WORKLOAD-001" }

func (WORKLOAD001) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	if sc == nil || sc.K8s == nil {
		return nil, nil
	}

	var issues []workloadPodIssue
	for _, pod := range sc.K8s.Pods {
		if issue, ok := unhealthyPodIssue(pod, time.Now().UTC()); ok {
			issues = append(issues, issue)
		}
	}
	sort.Slice(issues, func(i, j int) bool {
		a, b := issues[i], issues[j]
		if a.namespace != b.namespace {
			return a.namespace < b.namespace
		}
		if a.podName != b.podName {
			return a.podName < b.podName
		}
		if a.container != b.container {
			return a.container < b.container
		}
		return a.reason < b.reason
	})

	out := make([]findings.Finding, 0, len(issues))
	for _, issue := range issues {
		out = append(out, workload001Finding(issue, targetVersion))
	}
	return out, nil
}

type workloadPodIssue struct {
	pod          corev1.Pod
	namespace    string
	podName      string
	phase        corev1.PodPhase
	container    string
	reason       string
	ready        bool
	restartCount int32
}

func unhealthyPodIssue(pod corev1.Pod, now time.Time) (workloadPodIssue, bool) {
	if pod.DeletionTimestamp != nil || pod.Status.Phase == corev1.PodSucceeded {
		return workloadPodIssue{}, false
	}

	base := workloadPodIssue{
		pod:       pod,
		namespace: pod.Namespace,
		podName:   pod.Name,
		phase:     pod.Status.Phase,
		ready:     podReady(pod),
	}

	for _, status := range pod.Status.InitContainerStatuses {
		if issue, ok := unhealthyContainerIssue(base, status); ok {
			return issue, true
		}
	}
	for _, status := range pod.Status.ContainerStatuses {
		if issue, ok := unhealthyContainerIssue(base, status); ok {
			return issue, true
		}
	}

	switch pod.Status.Phase {
	case corev1.PodPending:
		base.reason = string(corev1.PodPending)
		if pod.Status.Reason != "" {
			base.reason = pod.Status.Reason
		}
		return base, true
	case corev1.PodFailed:
		base.reason = string(corev1.PodFailed)
		if pod.Status.Reason != "" {
			base.reason = pod.Status.Reason
		}
		return base, true
	case corev1.PodUnknown:
		base.reason = string(corev1.PodUnknown)
		if pod.Status.Reason != "" {
			base.reason = pod.Status.Reason
		}
		return base, true
	case corev1.PodRunning:
		if !base.ready && oldEnoughForNotReady(pod, now) {
			base.reason = "NotReady"
			return base, true
		}
	}
	return workloadPodIssue{}, false
}

func unhealthyContainerIssue(base workloadPodIssue, status corev1.ContainerStatus) (workloadPodIssue, bool) {
	issue := base
	issue.container = status.Name
	issue.ready = status.Ready
	issue.restartCount = status.RestartCount

	if waiting := status.State.Waiting; waiting != nil {
		reason := waiting.Reason
		if reason == "" {
			reason = "Waiting"
		}
		if _, ok := workloadUnhealthyWaitingReasons[reason]; ok {
			issue.reason = reason
			return issue, true
		}
	}

	if terminated := status.State.Terminated; terminated != nil && terminated.ExitCode != 0 {
		reason := terminated.Reason
		if reason == "" {
			reason = "Terminated"
		}
		issue.reason = reason
		return issue, true
	}
	return workloadPodIssue{}, false
}

func podReady(pod corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func oldEnoughForNotReady(pod corev1.Pod, now time.Time) bool {
	if pod.CreationTimestamp.IsZero() {
		return true
	}
	return now.Sub(pod.CreationTimestamp.Time) >= workloadNotReadyGracePeriod
}

func workload001Finding(issue workloadPodIssue, targetVersion string) findings.Finding {
	reason := issue.reason
	if reason == "" {
		reason = "Unhealthy"
	}

	msg := fmt.Sprintf(
		"Workload has unhealthy pods before upgrade: 1 pod in %s. This workload was unhealthy before the upgrade, which can make post-upgrade validation ambiguous.",
		reason)
	remediation := "Inspect the unhealthy workload before the upgrade. Confirm whether this is an expected pre-existing condition or a real application issue. Fix image references, pull secrets, config errors, or failing containers before the change window, or document an explicit waiver in the change ticket."

	ref := findings.LiveResource("Pod", findings.ScopeNamespaced, issue.namespace, issue.podName, string(issue.pod.UID))
	return findings.Finding{
		RuleID:      "WORKLOAD-001",
		Severity:    findings.SeverityWarning,
		Confidence:  findings.TierObserved,
		Message:     msg,
		Resources:   []findings.ResourceReference{ref},
		Evidence:    workload001Evidence(issue),
		Remediation: remediation,
		RemediationDetail: &findings.RemediationDetail{
			SafeFix: &findings.RemediationAction{
				Label: "Inspect workload health",
				Steps: []string{
					"Inspect the unhealthy pod and related events before the upgrade window.",
					"Fix image references, pull secrets, config errors, or failing containers, or document an explicit waiver in the change ticket.",
				},
				Command: strings.Join([]string{
					fmt.Sprintf("kubectl get pods -n %s", shellQuote(issue.namespace)),
					fmt.Sprintf("kubectl describe pod %s -n %s", shellQuote(issue.podName), shellQuote(issue.namespace)),
					fmt.Sprintf("kubectl get events -n %s --sort-by=.lastTimestamp", shellQuote(issue.namespace)),
					fmt.Sprintf("kubectl logs %s -n %s --all-containers --tail=100", shellQuote(issue.podName), shellQuote(issue.namespace)),
				}, "\n"),
			},
			VerifyCommand:  fmt.Sprintf("kubectl get pod %s -n %s -o jsonpath='{.status.phase}'", shellQuote(issue.podName), shellQuote(issue.namespace)),
			ExpectedResult: "pod is Running and Ready, Succeeded for an expected completed job, or explicitly waived before the change window",
		},
		Fingerprint: findings.FingerprintV2("WORKLOAD-001", targetVersion, "", ref),
	}
}

func workload001Evidence(issue workloadPodIssue) []string {
	evidence := []string{
		fmt.Sprintf("namespace: %s", issue.namespace),
		fmt.Sprintf("pod: %s", issue.podName),
		fmt.Sprintf("phase: %s", issue.phase),
	}
	if issue.container != "" {
		evidence = append(evidence, fmt.Sprintf("container: %s", issue.container))
	}
	evidence = append(evidence,
		fmt.Sprintf("reason: %s", issue.reason),
		fmt.Sprintf("ready: %t", issue.ready),
	)
	if issue.container != "" {
		evidence = append(evidence, fmt.Sprintf("restartCount: %d", issue.restartCount))
	}
	return evidence
}
