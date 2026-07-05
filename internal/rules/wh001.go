package rules

import (
	"fmt"
	"slices"
	"strings"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
			out = append(out, wh001Finding(wh001Params{
				Kind: "ValidatingWebhookConfiguration", PatchResource: "validatingwebhookconfiguration",
				ConfigName: cfg.Name, ConfigUID: string(cfg.UID),
				WebhookName: wh.Name, ResourcePattern: pattern,
				Operations:           catchAllOperations(wh.Rules),
				FailurePolicySet:     wh.FailurePolicy != nil,
				HasNamespaceSelector: hasSelector(wh.NamespaceSelector),
				HasObjectSelector:    hasSelector(wh.ObjectSelector),
				HasMatchConditions:   len(wh.MatchConditions) > 0,
			}, targetVersion))
		}
	}

	for _, cfg := range snap.MutatingWebhookConfigs {
		for _, wh := range cfg.Webhooks {
			matched, pattern := hasCatchAllRule(wh.Rules)
			if !isFailClosed(wh.FailurePolicy) || !matched {
				continue
			}
			out = append(out, wh001Finding(wh001Params{
				Kind: "MutatingWebhookConfiguration", PatchResource: "mutatingwebhookconfiguration",
				ConfigName: cfg.Name, ConfigUID: string(cfg.UID),
				WebhookName: wh.Name, ResourcePattern: pattern,
				Operations:           catchAllOperations(wh.Rules),
				FailurePolicySet:     wh.FailurePolicy != nil,
				HasNamespaceSelector: hasSelector(wh.NamespaceSelector),
				HasObjectSelector:    hasSelector(wh.ObjectSelector),
				HasMatchConditions:   len(wh.MatchConditions) > 0,
			}, targetVersion))
		}
	}

	return out, nil
}

func catchAllOperations(rules []admissionregistrationv1.RuleWithOperations) string {
	for _, rule := range rules {
		matched, _ := hasCatchAllRule([]admissionregistrationv1.RuleWithOperations{rule})
		if !matched {
			continue
		}
		ops := make([]string, len(rule.Operations))
		for i, operation := range rule.Operations {
			ops[i] = string(operation)
		}
		return strings.Join(ops, ",")
	}
	return ""
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

// hasSelector reports whether a LabelSelector is meaningfully restrictive
// (nil or the zero-value selector both mean "select everything").
func hasSelector(sel *metav1.LabelSelector) bool {
	return sel != nil && (len(sel.MatchLabels) > 0 || len(sel.MatchExpressions) > 0)
}

type wh001Params struct {
	Kind, PatchResource          string
	ConfigName, ConfigUID        string
	WebhookName, ResourcePattern string
	Operations                   string
	FailurePolicySet             bool
	HasNamespaceSelector         bool
	HasObjectSelector            bool
	HasMatchConditions           bool
}

func wh001Finding(p wh001Params, targetVersion string) findings.Finding {
	scope := "all namespaces and cluster-scoped resources"
	if p.HasNamespaceSelector || p.HasObjectSelector || p.HasMatchConditions {
		scope = "requests that also satisfy its configured selectors/match conditions"
	}
	msg := fmt.Sprintf(
		"%s %q: webhook %q is fail-closed with catch-all resource rules (apiGroups: [\"*\"], resources: [%q], operations: [%s]) — %s depend on this webhook's backend being healthy",
		p.Kind, p.ConfigName, p.WebhookName, p.ResourcePattern, p.Operations, scope)

	remediation := "Narrow the webhook's rules to the specific apiGroups/resources it actually needs to validate/mutate, " +
		"and add a namespaceSelector excluding kube-system and other critical namespaces. " +
		"If this webhook does simple field validation, consider migrating it to a ValidatingAdmissionPolicy (CEL) " +
		"to remove the callback dependency entirely."

	ref := findings.LiveResource(p.Kind, findings.ScopeCluster, "", p.ConfigName, p.ConfigUID)
	return findings.Finding{
		RuleID:     "WH-001",
		Severity:   findings.SeverityWarning,
		Confidence: findings.TierStaticCertain,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		Evidence: []string{
			fmt.Sprintf("webhook name: %s", p.WebhookName),
			fmt.Sprintf("scope: apiGroups=[\"*\"], resources=[%q]", p.ResourcePattern),
			fmt.Sprintf("operations: [%s]", p.Operations),
			fmt.Sprintf("failurePolicy: %s", failurePolicyLiteral(p.FailurePolicySet)),
			fmt.Sprintf("namespaceSelector set: %t", p.HasNamespaceSelector),
			fmt.Sprintf("objectSelector set: %t", p.HasObjectSelector),
			fmt.Sprintf("matchConditions set: %t", p.HasMatchConditions),
		},
		Remediation:       remediation,
		RemediationDetail: wh001RemediationDetail(p),
		Fingerprint:       findings.FingerprintV2("WH-001", targetVersion, p.WebhookName, ref),
	}
}

func wh001RemediationDetail(p wh001Params) *findings.RemediationDetail {
	return &findings.RemediationDetail{
		SafeFix: &findings.RemediationAction{
			Label: "Safe fix",
			Steps: []string{
				"Narrow the webhook's rules to the specific apiGroups/resources it actually needs to validate/mutate.",
				"Add a namespaceSelector excluding kube-system and other critical namespaces.",
				"If this webhook only does simple field validation, consider migrating it to a ValidatingAdmissionPolicy (CEL) to remove the callback dependency entirely.",
			},
		},
	}
}
