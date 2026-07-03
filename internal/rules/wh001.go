package rules

import (
	"fmt"
	"slices"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"

	"kubepreflight/internal/findings"
)

// WH001 flags a fail-closed admission webhook with catch-all scope
// (apiGroups: ["*"], resources: ["*"]) on at least one rule: even
// kube-system and node-lifecycle writes go through it. This is a Warning on
// its own — WH-002 is what escalates the same webhook to a Blocker once its
// backend is also observed unhealthy (deep dive Section 5, check WH-001).
type WH001 struct{}

func (WH001) ID() string { return "WH-001" }

func (WH001) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	snap := sc.K8s
	var out []findings.Finding

	for _, cfg := range snap.ValidatingWebhookConfigs {
		for _, wh := range cfg.Webhooks {
			if !isFailClosed(wh.FailurePolicy) || !hasCatchAllRule(wh.Rules) {
				continue
			}
			out = append(out, wh001Finding("ValidatingWebhookConfiguration", cfg.Name, string(cfg.UID), wh.Name, targetVersion))
		}
	}

	for _, cfg := range snap.MutatingWebhookConfigs {
		for _, wh := range cfg.Webhooks {
			if !isFailClosed(wh.FailurePolicy) || !hasCatchAllRule(wh.Rules) {
				continue
			}
			out = append(out, wh001Finding("MutatingWebhookConfiguration", cfg.Name, string(cfg.UID), wh.Name, targetVersion))
		}
	}

	return out, nil
}

// hasCatchAllRule reports whether any rule in the webhook matches every API
// group and every resource — the broadest possible scope.
func hasCatchAllRule(rules []admissionregistrationv1.RuleWithOperations) bool {
	for _, r := range rules {
		if slices.Contains(r.APIGroups, "*") && slices.Contains(r.Resources, "*") {
			return true
		}
	}
	return false
}

func wh001Finding(kind, name, uid, webhookName, targetVersion string) findings.Finding {
	msg := fmt.Sprintf(
		"%s %q: webhook %q is fail-closed with catch-all scope (apiGroups: [\"*\"], resources: [\"*\"]) — every matching write in the cluster, including kube-system objects, depends on this webhook's backend being healthy",
		kind, name, webhookName)

	remediation := "Narrow the webhook's rules to the specific apiGroups/resources it actually needs to validate/mutate, " +
		"and add a namespaceSelector excluding kube-system and other critical namespaces. " +
		"If this webhook does simple field validation, consider migrating it to a ValidatingAdmissionPolicy (CEL) " +
		"to remove the callback dependency entirely."

	return findings.Finding{
		RuleID:     "WH-001",
		Severity:   findings.SeverityWarning,
		Confidence: findings.TierStaticCertain,
		Message:    msg,
		Resource: findings.Resource{
			Kind: kind,
			Name: name,
			UID:  uid,
		},
		Evidence: []string{
			fmt.Sprintf("webhook name: %s", webhookName),
			"scope: apiGroups=[\"*\"], resources=[\"*\"]",
			"failurePolicy: Fail (or unset, which defaults to Fail)",
		},
		Remediation: remediation,
		Fingerprint: findings.Fingerprint("WH-001", fmt.Sprintf("%s/%s", uid, webhookName), targetVersion),
	}
}
