package rules

import (
	"fmt"
	"slices"
	"strings"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

// wh005ExcessiveTimeoutSeconds is the threshold (inclusive) above which a
// webhook's timeoutSeconds is flagged as excessive: within 5 seconds of the
// API server's hard 30-second ceiling (admissionregistration.k8s.io/v1
// enforces 1-30). A stalled backend at this timeout can make matching
// requests hang for nearly the full window.
const wh005ExcessiveTimeoutSeconds = 25

// wh005ResourceTarget is one (apiGroup, resource) pair worth flagging if a
// webhook's rules cover it.
type wh005ResourceTarget struct {
	APIGroup string
	Resource string
}

// wh005SelfInterceptionTargets are the webhook config resources
// themselves. A fail-closed webhook whose rules also cover these can block
// the very fix (kubectl patch/delete on the VWC/MWC) that would repair it
// — a well-known admission-webhook footgun.
var wh005SelfInterceptionTargets = []wh005ResourceTarget{
	{APIGroup: "admissionregistration.k8s.io", Resource: "validatingwebhookconfigurations"},
	{APIGroup: "admissionregistration.k8s.io", Resource: "mutatingwebhookconfigurations"},
}

// wh005HighRiskTargets are cluster-critical resources whose write path
// being intercepted by a fail-closed webhook is a well-known operational
// hazard: blocking Node status updates during a drain/upgrade, Namespace
// lifecycle operations, or PersistentVolume writes can wedge cluster
// maintenance in ways that are hard to diagnose from the symptom alone.
var wh005HighRiskTargets = []wh005ResourceTarget{
	{APIGroup: "", Resource: "nodes"},
	{APIGroup: "", Resource: "namespaces"},
	{APIGroup: "", Resource: "persistentvolumes"},
}

// WH005 flags admission webhook scope/timeout configurations that are
// risky independent of current backend health (WH-002) or catch-all scope
// (WH-001): an excessive timeoutSeconds, operations: ["*"] (which includes
// CONNECT — exec/attach/portforward/proxy subresources), a webhook
// intercepting writes to admission webhook configs themselves, or a
// fail-closed webhook covering cluster-critical resources (nodes,
// namespaces, persistentvolumes).
//
// Not every broad webhook is a Blocker: general scope risk (excessive
// timeout, wildcard operations) stays a Warning regardless of
// failurePolicy, since nothing is currently broken. Self-interception and
// fail-closed high-risk-resource coverage escalate to Blocker, since both
// describe a webhook that can actively wedge cluster operations (and, for
// self-interception, wedge its own remediation) right now.
type WH005 struct{}

func (WH005) ID() string { return "WH-005" }

func (WH005) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	snap := sc.K8s
	if snap == nil {
		return nil, nil
	}
	var out []findings.Finding

	for _, cfg := range snap.ValidatingWebhookConfigs {
		for i, wh := range cfg.Webhooks {
			out = append(out, wh005EvaluateWebhook(wh005Input{
				Kind:       "ValidatingWebhookConfiguration",
				ConfigName: cfg.Name, ConfigUID: string(cfg.UID),
				WebhookName: wh.Name, WebhookIndex: i,
				FailurePolicy: wh.FailurePolicy, TimeoutSeconds: wh.TimeoutSeconds,
				Rules: wh.Rules,
			}, targetVersion)...)
		}
	}
	for _, cfg := range snap.MutatingWebhookConfigs {
		for i, wh := range cfg.Webhooks {
			out = append(out, wh005EvaluateWebhook(wh005Input{
				Kind:       "MutatingWebhookConfiguration",
				ConfigName: cfg.Name, ConfigUID: string(cfg.UID),
				WebhookName: wh.Name, WebhookIndex: i,
				FailurePolicy: wh.FailurePolicy, TimeoutSeconds: wh.TimeoutSeconds,
				Rules: wh.Rules,
			}, targetVersion)...)
		}
	}
	return out, nil
}

type wh005Input struct {
	Kind                  string
	ConfigName, ConfigUID string
	WebhookName           string
	WebhookIndex          int
	FailurePolicy         *admissionregistrationv1.FailurePolicyType
	TimeoutSeconds        *int32
	Rules                 []admissionregistrationv1.RuleWithOperations
}

func wh005EvaluateWebhook(in wh005Input, targetVersion string) []findings.Finding {
	ref := findings.LiveResource(in.Kind, findings.ScopeCluster, "", in.ConfigName, in.ConfigUID)
	failClosed := isFailClosed(in.FailurePolicy)
	var out []findings.Finding

	if in.TimeoutSeconds != nil && *in.TimeoutSeconds >= wh005ExcessiveTimeoutSeconds {
		out = append(out, wh005Finding(in, ref, findings.SeverityWarning, false, false, "excessive-timeout", targetVersion,
			fmt.Sprintf("%s %q: webhook %q (index %d in .webhooks) has timeoutSeconds=%d, within %d seconds of the API server's 30-second ceiling — a slow or stalled backend can make matching requests hang for nearly that long", in.Kind, in.ConfigName, in.WebhookName, in.WebhookIndex, *in.TimeoutSeconds, 30-*in.TimeoutSeconds),
			[]string{fmt.Sprintf("webhook name: %s", in.WebhookName), fmt.Sprintf("timeoutSeconds: %d", *in.TimeoutSeconds)},
			"Lower timeoutSeconds to a value that reflects the backend's real p99 latency (commonly 5-10s), so a stalled backend fails fast instead of holding up the request for the full window.",
		))
	}

	if resourcePattern, ok := wh005WildcardOperationRule(in.Rules); ok {
		out = append(out, wh005Finding(in, ref, findings.SeverityWarning, false, false, "wildcard-operations", targetVersion,
			fmt.Sprintf("%s %q: webhook %q (index %d in .webhooks) matches operations: [\"*\"] on resources %q — this includes CONNECT (exec/attach/portforward/proxy subresources), which most validating/mutating webhooks never need to intercept", in.Kind, in.ConfigName, in.WebhookName, in.WebhookIndex, resourcePattern),
			[]string{fmt.Sprintf("webhook name: %s", in.WebhookName), fmt.Sprintf("operations: [\"*\"] on resources %q", resourcePattern)},
			"Replace operations: [\"*\"] with the specific operations this webhook actually needs (typically CREATE and/or UPDATE).",
		))
	}

	if matched, resource := wh005MatchesTargets(in.Rules, wh005SelfInterceptionTargets); matched {
		out = append(out, wh005Finding(in, ref, wh005ScopeSeverity(failClosed), failClosed, false, "self-interception:"+resource, targetVersion,
			fmt.Sprintf("%s %q: webhook %q (index %d in .webhooks) matches %s — this webhook can intercept writes to admission webhook configs, including attempts to fix or disable itself", in.Kind, in.ConfigName, in.WebhookName, in.WebhookIndex, resource),
			[]string{fmt.Sprintf("webhook name: %s", in.WebhookName), fmt.Sprintf("matched resource: %s", resource)},
			"Exclude admissionregistration.k8s.io (validatingwebhookconfigurations/mutatingwebhookconfigurations) from this webhook's rules, so a misbehaving webhook can always be patched or deleted.",
		))
	}

	if matched, resource := wh005MatchesTargets(in.Rules, wh005HighRiskTargets); matched {
		out = append(out, wh005Finding(in, ref, wh005ScopeSeverity(failClosed), false, failClosed, "high-risk-resource-scope:"+resource, targetVersion,
			fmt.Sprintf("%s %q: webhook %q (index %d in .webhooks) matches %s — a fail-closed webhook here can block node status updates, namespace lifecycle, or PersistentVolume operations that upgrade/maintenance workflows depend on", in.Kind, in.ConfigName, in.WebhookName, in.WebhookIndex, resource),
			[]string{fmt.Sprintf("webhook name: %s", in.WebhookName), fmt.Sprintf("matched resource: %s", resource)},
			"Confirm this webhook genuinely needs to validate/mutate this resource. If not, narrow its rules to exclude it.",
		))
	}

	return out
}

func wh005ScopeSeverity(failClosed bool) findings.Severity {
	if failClosed {
		return findings.SeverityBlocker
	}
	return findings.SeverityWarning
}

// wh005WildcardOperationRule reports whether any rule matches
// operations: ["*"], and if so, the resource pattern it applies to (for
// the finding's evidence). Per the API's own validation, '*' must be the
// only entry in Operations when present, so checking the first element is
// sufficient.
func wh005WildcardOperationRule(rules []admissionregistrationv1.RuleWithOperations) (resourcePattern string, ok bool) {
	for _, r := range rules {
		if len(r.Operations) == 1 && r.Operations[0] == admissionregistrationv1.OperationAll {
			return strings.Join(r.Resources, ","), true
		}
	}
	return "", false
}

// wh005MatchesTargets reports whether any rule covers one of targets,
// honoring both a wildcard apiGroup ("*") and wildcard resources ("*" or
// "*/*"). Returns the first target matched, since one finding per category
// per webhook is enough evidence to act on.
func wh005MatchesTargets(rules []admissionregistrationv1.RuleWithOperations, targets []wh005ResourceTarget) (matched bool, resource string) {
	for _, r := range rules {
		groupWildcard := slices.Contains(r.APIGroups, "*")
		resourceWildcard := slices.Contains(r.Resources, "*") || slices.Contains(r.Resources, "*/*")
		for _, t := range targets {
			if (groupWildcard || slices.Contains(r.APIGroups, t.APIGroup)) &&
				(resourceWildcard || slices.Contains(r.Resources, t.Resource)) {
				return true, t.Resource
			}
		}
	}
	return false, ""
}

func wh005Finding(in wh005Input, ref findings.ResourceReference, severity findings.Severity, global, criticalInfra bool, discriminator, targetVersion, message string, evidence []string, remediation string) findings.Finding {
	return findings.Finding{
		RuleID:        "WH-005",
		Severity:      severity,
		Confidence:    findings.TierStaticCertain,
		Message:       message,
		Resources:     []findings.ResourceReference{ref},
		Evidence:      append(evidence, fmt.Sprintf("failurePolicy: %s", failurePolicyLiteral(in.FailurePolicy))),
		Remediation:   remediation,
		GlobalBlocker: global,
		CriticalInfra: criticalInfra,
		Fingerprint:   findings.FingerprintV2("WH-005", targetVersion, in.WebhookName+":"+discriminator, ref),
	}
}
