package rules

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"kubepreflight/internal/findings"
)

// COREDNS001 flags a CoreDNS Corefile missing the `ready` plugin: without
// it, the CoreDNS pod's readiness probe can't reflect actual DNS server
// health, which is a documented EKS trap that tends to surface only after
// an add-on update (deep dive Section 9.2, rides the ADDON collector per
// the locked MVP scope in Section 18.2).
type COREDNS001 struct{}

func (COREDNS001) ID() string { return "COREDNS-001" }

func (COREDNS001) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	if sc.K8s == nil || sc.K8s.CoreDNSConfigMap == nil {
		return nil, nil // no CoreDNS ConfigMap found — nothing to check
	}

	corefile, ok := sc.K8s.CoreDNSConfigMap.Data["Corefile"]
	if !ok || hasReadyPlugin(corefile) {
		return nil, nil
	}

	return []findings.Finding{coredns001Finding(sc.K8s.CoreDNSConfigMap, targetVersion)}, nil
}

// hasReadyPlugin reports whether any line in the Corefile has `ready` as
// its first token — the plugin directive, not a substring match (so
// "readyz" or a comment mentioning "ready" doesn't false-positive).
func hasReadyPlugin(corefile string) bool {
	for _, line := range strings.Split(corefile, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == "ready" {
			return true
		}
	}
	return false
}

func coredns001Finding(cm *corev1.ConfigMap, targetVersion string) findings.Finding {
	msg := fmt.Sprintf(
		"CoreDNS Corefile (%s/%s) is missing the `ready` plugin — the CoreDNS pod's readiness probe can't reflect actual DNS server health, so a pod can be marked Ready before CoreDNS is actually serving, most likely to surface right after an add-on update",
		cm.Namespace, cm.Name)

	remediation := "Add `ready` as a standalone directive inside the server block (typically alongside `health`). " +
		"Back up the Corefile ConfigMap first, then apply the change directly with `kubectl apply` or via " +
		"`aws eks update-addon --addon-name coredns --resolve-conflicts PRESERVE` if CoreDNS is managed as an EKS add-on."

	return findings.Finding{
		RuleID:     "COREDNS-001",
		Severity:   findings.SeverityWarning,
		Confidence: findings.TierStaticCertain,
		Message:    msg,
		Resource: findings.Resource{
			Kind:      "ConfigMap",
			Namespace: cm.Namespace,
			Name:      cm.Name,
			UID:       string(cm.UID),
		},
		Evidence: []string{
			"Corefile has no standalone `ready` directive",
		},
		Remediation: remediation,
		Fingerprint: findings.Fingerprint("COREDNS-001", string(cm.UID), targetVersion),
	}
}
