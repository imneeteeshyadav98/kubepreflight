package rules

import (
	"fmt"
	"slices"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"

	"kubepreflight/internal/findings"
)

// WH001 flags a fail-closed admission webhook with catch-all scope
// (apiGroups: ["*"], resources matching every resource) on at least one
// rule: even kube-system and node-lifecycle writes go through it. This is
// a Warning on its own — WH-002 is what escalates the same webhook to a
// Blocker once its backend is also observed unhealthy (deep dive Section
// 5, check WH-001).
type WH001 struct{}

func (WH001) ID() string { return "WH-001" }

func (WH001) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	snap := sc.K8s
	var out []findings.Finding

	for _, cfg := range snap.ValidatingWebhookConfigs {
		for _, wh := range cfg.Webhooks {
			matched, pattern := hasCatchAllRule(wh.Rules)
			if !isFailClosed(wh.FailurePolicy) || !matched {
				continue
			}
			out = append(out, wh001Finding("ValidatingWebhookConfiguration", cfg.Name, string(cfg.UID), wh.Name, pattern, targetVersion))
		}
	}

	for _, cfg := range snap.MutatingWebhookConfigs {
		for _, wh := range cfg.Webhooks {
			matched, pattern := hasCatchAllRule(wh.Rules)
			if !isFailClosed(wh.FailurePolicy) || !matched {
				continue
			}
			out = append(out, wh001Finding("MutatingWebhookConfiguration", cfg.Name, string(cfg.UID), wh.Name, pattern, targetVersion))
		}
	}

	return out, nil
}

// hasCatchAllRule reports whether any rule in the webhook matches every API
// group and every resource — the broadest possible scope — and if so,
// which resource pattern triggered it. Kubernetes treats two different
// spellings as "every resource" for admission webhook rules: "*" matches
// all resources but NOT subresources, and "*/*" matches all resources AND
// all subresources (e.g. pods/status, pods/exec) — a real, commonly
// generated pattern (found via an actual live-cluster test, not a
// hypothetical) that a literal-"*"-only check silently missed.
func hasCatchAllRule(rules []admissionregistrationv1.RuleWithOperations) (matched bool, resourcePattern string) {
	for _, r := range rules {
		if !slices.Contains(r.APIGroups, "*") {
			continue
		}
		for _, res := range r.Resources {
			if res == "*" || res == "*/*" {
				return true, res
			}
		}
	}
	return false, ""
}

func wh001Finding(kind, name, uid, webhookName, resourcePattern, targetVersion string) findings.Finding {
	msg := fmt.Sprintf(
		"%s %q: webhook %q is fail-closed with catch-all scope (apiGroups: [\"*\"], resources: [%q]) — every matching write in the cluster, including kube-system objects, depends on this webhook's backend being healthy",
		kind, name, webhookName, resourcePattern)

	remediation := "Narrow the webhook's rules to the specific apiGroups/resources it actually needs to validate/mutate, " +
		"and add a namespaceSelector excluding kube-system and other critical namespaces. " +
		"If this webhook does simple field validation, consider migrating it to a ValidatingAdmissionPolicy (CEL) " +
		"to remove the callback dependency entirely."

	ref := findings.LiveResource(kind, findings.ScopeCluster, "", name, uid)
	return findings.Finding{
		RuleID:     "WH-001",
		Severity:   findings.SeverityWarning,
		Confidence: findings.TierStaticCertain,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		Evidence: []string{
			fmt.Sprintf("webhook name: %s", webhookName),
			fmt.Sprintf("scope: apiGroups=[\"*\"], resources=[%q]", resourcePattern),
			"failurePolicy: Fail (or unset, which defaults to Fail)",
		},
		Remediation: remediation,
		Fingerprint: findings.FingerprintV2("WH-001", targetVersion, webhookName, ref),
	}
}
