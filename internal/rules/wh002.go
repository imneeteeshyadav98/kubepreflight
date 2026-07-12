package rules

import (
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubepreflight/internal/collectors/k8s"
	"kubepreflight/internal/findings"
)

// WH002 flags an admission webhook whose backend is unavailable or
// structurally broken: a missing/invalid clientConfig, a referenced
// Service that doesn't exist, an out-of-range port, or zero ready
// endpoint addresses. Severity depends on failurePolicy, since the two
// have fundamentally different consequences:
//
//   - Fail (or unset, which defaults to Fail): every matching API write is
//     rejected until the backend recovers -- Blocker. When the same
//     webhook also has catch-all scope (WH-001's condition), it's flagged
//     GlobalBlocker — its outage doesn't just fail its own writes, it can
//     fail kubectl/Helm remediation for anything else in the cluster too.
//   - Ignore: matching writes are silently admitted without going through
//     the webhook at all -- no write is rejected, so this can't be a
//     GlobalBlocker, but admission control silently not applying is still
//     worth surfacing -- Warning.
type WH002 struct{}

func (WH002) ID() string { return "WH-002" }

func (WH002) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	snap := sc.K8s
	if snap == nil {
		return nil, nil
	}
	if _, unavailable := snap.Errors["endpointslices"]; unavailable {
		return nil, nil
	}
	var out []findings.Finding

	for _, cfg := range snap.ValidatingWebhookConfigs {
		for i, wh := range cfg.Webhooks {
			out = append(out, wh002EvaluateWebhook(snap, wh002Input{
				Kind: "ValidatingWebhookConfiguration", PatchResource: "validatingwebhookconfiguration",
				ConfigName: cfg.Name, ConfigUID: string(cfg.UID),
				WebhookName: wh.Name, WebhookIndex: i,
				FailurePolicy: wh.FailurePolicy, ClientConfig: wh.ClientConfig,
				Rules: wh.Rules, NamespaceSelector: wh.NamespaceSelector, ObjectSelector: wh.ObjectSelector,
				HasMatchConditions: len(wh.MatchConditions) > 0,
			}, targetVersion)...)
		}
	}

	for _, cfg := range snap.MutatingWebhookConfigs {
		for i, wh := range cfg.Webhooks {
			out = append(out, wh002EvaluateWebhook(snap, wh002Input{
				Kind: "MutatingWebhookConfiguration", PatchResource: "mutatingwebhookconfiguration",
				ConfigName: cfg.Name, ConfigUID: string(cfg.UID),
				WebhookName: wh.Name, WebhookIndex: i,
				FailurePolicy: wh.FailurePolicy, ClientConfig: wh.ClientConfig,
				Rules: wh.Rules, NamespaceSelector: wh.NamespaceSelector, ObjectSelector: wh.ObjectSelector,
				HasMatchConditions: len(wh.MatchConditions) > 0,
			}, targetVersion)...)
		}
	}

	return out, nil
}

type wh002Input struct {
	Kind, PatchResource   string
	ConfigName, ConfigUID string
	WebhookName           string
	WebhookIndex          int
	FailurePolicy         *admissionregistrationv1.FailurePolicyType
	ClientConfig          admissionregistrationv1.WebhookClientConfig
	Rules                 []admissionregistrationv1.RuleWithOperations
	NamespaceSelector     *metav1.LabelSelector
	ObjectSelector        *metav1.LabelSelector
	HasMatchConditions    bool
}

// wh002EvaluateWebhook is the single per-webhook decision point shared by
// both ValidatingWebhookConfiguration and MutatingWebhookConfiguration --
// their per-webhook types differ, but ClientConfig/Rules/FailurePolicy/
// selectors are the same shared admissionregistration types, so extracting
// the fields once at the call site (above) lets this run unconditionally
// regardless of failurePolicy, unlike the old Fail-only early skip.
func wh002EvaluateWebhook(snap *k8s.Snapshot, in wh002Input, targetVersion string) []findings.Finding {
	failClosed := isFailClosed(in.FailurePolicy)
	ref := findings.LiveResource(in.Kind, findings.ScopeCluster, "", in.ConfigName, in.ConfigUID)
	// Depends only on Rules/selectors/failurePolicy, not on Service/endpoint
	// state, so it's the same regardless of which case below actually
	// fires -- a catch-all-scope Fail-closed webhook is equally a global
	// blocker whether its backend is missing entirely, misconfigured, or
	// merely unhealthy.
	global := failClosed && hasGlobalWriteScope(in.Rules, in.NamespaceSelector, in.ObjectSelector, in.HasMatchConditions)

	if in.ClientConfig.Service == nil && in.ClientConfig.URL == nil {
		return []findings.Finding{wh002ConfigFinding(in, ref, failClosed, global, "no-client-config", targetVersion,
			fmt.Sprintf("%s %q: webhook %q (index %d in .webhooks) has neither a service nor a url in clientConfig — the API server has no way to reach it", in.Kind, in.ConfigName, in.WebhookName, in.WebhookIndex),
			[]string{fmt.Sprintf("webhook name: %s", in.WebhookName), "clientConfig.service: not set", "clientConfig.url: not set"},
		)}
	}

	if in.ClientConfig.Service == nil {
		// URL-based clientConfig: left unvalidated beyond "is it set" --
		// this tool doesn't probe arbitrary external endpoints, matching
		// the same boundary internal/rules/crd002.go draws for conversion
		// webhooks.
		return nil
	}
	svc := in.ClientConfig.Service

	if svc.Port != nil && (*svc.Port < 1 || *svc.Port > 65535) {
		return []findings.Finding{wh002ConfigFinding(in, ref, failClosed, global, "invalid-port", targetVersion,
			fmt.Sprintf("%s %q: webhook %q (index %d in .webhooks) references service %s/%s on invalid port %d — a port must be between 1 and 65535", in.Kind, in.ConfigName, in.WebhookName, in.WebhookIndex, svc.Namespace, svc.Name, *svc.Port),
			[]string{fmt.Sprintf("webhook name: %s", in.WebhookName), fmt.Sprintf("service: %s/%s", svc.Namespace, svc.Name), fmt.Sprintf("port: %d", *svc.Port)},
		)}
	}

	if !serviceExists(snap, svc.Namespace, svc.Name) {
		return []findings.Finding{wh002ConfigFinding(in, ref, failClosed, global, "service-not-found", targetVersion,
			fmt.Sprintf("%s %q: webhook %q (index %d in .webhooks) references service %s/%s, which does not exist in this cluster", in.Kind, in.ConfigName, in.WebhookName, in.WebhookIndex, svc.Namespace, svc.Name),
			[]string{fmt.Sprintf("webhook name: %s", in.WebhookName), fmt.Sprintf("service: %s/%s", svc.Namespace, svc.Name), "service object: not found"},
		)}
	}

	if readyAddressCount(snap, svc.Namespace, svc.Name) > 0 {
		return nil
	}

	if failClosed {
		return []findings.Finding{wh002Finding(wh002Params{
			Kind: in.Kind, PatchResource: in.PatchResource,
			ConfigName: in.ConfigName, ConfigUID: in.ConfigUID,
			WebhookName: in.WebhookName, WebhookIndex: in.WebhookIndex,
			FailurePolicySet: in.FailurePolicy != nil, FailurePolicy: in.FailurePolicy,
			SvcNamespace: svc.Namespace, SvcName: svc.Name,
			GlobalBlocker: global,
		}, targetVersion)}
	}
	return []findings.Finding{wh002IgnoreFinding(in, ref, svc, targetVersion)}
}

// serviceExists reports whether a Service object with this namespace/name
// was actually observed in the cluster snapshot, as opposed to merely
// having zero ready endpoints -- distinguishing "nothing was ever created"
// from "it exists but is unhealthy" gives a much clearer remediation
// starting point. Shared with internal/rules/crd002.go's identical need.
func serviceExists(snap *k8s.Snapshot, namespace, name string) bool {
	for _, svc := range snap.Services {
		if svc.Namespace == namespace && svc.Name == name {
			return true
		}
	}
	return false
}

// wh002ConfigFinding builds a finding for a static webhook misconfiguration
// (as opposed to the live endpoint-health check, which carries its own
// richer RemediationDetail). Severity follows failClosed: a Fail-policy
// webhook with a broken backend rejects every matching write (Blocker); an
// Ignore-policy webhook with the same broken backend silently admits
// writes without validation/mutation (Warning) -- not a hard blocker, but
// silent admission control not applying is still worth surfacing.
func wh002ConfigFinding(in wh002Input, ref findings.ResourceReference, failClosed, global bool, discriminator, targetVersion, message string, evidence []string) findings.Finding {
	severity := findings.SeverityWarning
	remediation := "This webhook's failurePolicy is Ignore, so matching API writes are currently admitted without going through it — fix the clientConfig so admission control actually applies, or remove the webhook if it's no longer needed."
	if failClosed {
		severity = findings.SeverityBlocker
		remediation = "Fix the webhook's clientConfig before upgrading — every matching API write is currently rejected."
	}
	return findings.Finding{
		RuleID:        "WH-002",
		Severity:      severity,
		Confidence:    findings.TierStaticCertain,
		Message:       message,
		Resources:     []findings.ResourceReference{ref},
		Evidence:      append(evidence, fmt.Sprintf("failurePolicy: %s", failurePolicyLiteral(in.FailurePolicy))),
		Remediation:   remediation,
		GlobalBlocker: global,
		Fingerprint:   findings.FingerprintV2("WH-002", targetVersion, in.WebhookName+":"+discriminator, ref),
	}
}

// wh002IgnoreFinding is the Warning-severity counterpart to wh002Finding
// (the original Fail-closed Blocker case, unchanged below): same
// zero-ready-endpoints condition, but nothing is actually rejected with
// failurePolicy Ignore, so there's no "temporarily set failurePolicy to
// Ignore" emergency mitigation to offer -- it already is.
func wh002IgnoreFinding(in wh002Input, ref findings.ResourceReference, svc *admissionregistrationv1.ServiceReference, targetVersion string) findings.Finding {
	msg := fmt.Sprintf(
		"%s %q: webhook %q (index %d in .webhooks) has failurePolicy Ignore and its backend service %s/%s has zero ready endpoints — matching API writes are currently admitted without going through this webhook at all",
		in.Kind, in.ConfigName, in.WebhookName, in.WebhookIndex, svc.Namespace, svc.Name)
	return findings.Finding{
		RuleID:     "WH-002",
		Severity:   findings.SeverityWarning,
		Confidence: findings.TierObserved,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		Evidence: []string{
			fmt.Sprintf("webhook name: %s", in.WebhookName),
			fmt.Sprintf("webhook index: %d", in.WebhookIndex),
			fmt.Sprintf("backend service: %s/%s", svc.Namespace, svc.Name),
			"ready endpoint address count: 0",
			fmt.Sprintf("failurePolicy: %s", failurePolicyLiteral(in.FailurePolicy)),
		},
		Remediation: fmt.Sprintf("Restore the webhook backend so admission control actually applies again:\n\nkubectl get svc %s -n %s\nkubectl get endpointslices -n %s -l kubernetes.io/service-name=%s\nkubectl get deploy,pods -n %s",
			shellQuote(svc.Name), shellQuote(svc.Namespace), shellQuote(svc.Namespace), shellQuote(svc.Name), shellQuote(svc.Namespace)),
		RemediationDetail: &findings.RemediationDetail{
			Changes:       []findings.RemediationChange{{Field: "endpoint count", Current: "0", Required: ">= 1"}},
			SafeFix:       &findings.RemediationAction{Label: "Safe fix", Steps: []string{"Restore the backend's health — with failurePolicy Ignore, no emergency mitigation is needed; writes already aren't being blocked."}, Command: fmt.Sprintf("kubectl get svc %s -n %s\nkubectl get endpointslices -n %s -l kubernetes.io/service-name=%s", shellQuote(svc.Name), shellQuote(svc.Namespace), shellQuote(svc.Namespace), shellQuote(svc.Name))},
			VerifyCommand: fmt.Sprintf("kubectl get endpointslices -n %s -l kubernetes.io/service-name=%s", shellQuote(svc.Namespace), shellQuote(svc.Name)), ExpectedResult: "endpoint count >= 1",
		},
		// Same discriminator shape as wh002ConfigFinding's new cases --
		// distinct from the Fail-closed case's bare-webhook-name
		// fingerprint below, since the two are different findings even
		// when they'd otherwise describe the same backend.
		Fingerprint: findings.FingerprintV2("WH-002", targetVersion, in.WebhookName+":ignore-zero-endpoints", ref),
	}
}

func hasGlobalWriteScope(rules []admissionregistrationv1.RuleWithOperations, namespaceSelector, objectSelector *metav1.LabelSelector, hasMatchConditions bool) bool {
	if hasSelector(namespaceSelector) || hasSelector(objectSelector) || hasMatchConditions {
		return false
	}
	for _, rule := range rules {
		catchAll, _ := hasCatchAllRule([]admissionregistrationv1.RuleWithOperations{rule})
		if !catchAll {
			continue
		}
		for _, operation := range rule.Operations {
			if operation == admissionregistrationv1.OperationAll || operation == admissionregistrationv1.Create || operation == admissionregistrationv1.Update {
				return true
			}
		}
	}
	return false
}

// isFailClosed reports whether a webhook's failurePolicy blocks requests
// when the webhook backend is unreachable. admissionregistration.k8s.io/v1
// defaults an unset FailurePolicy to Fail (v1beta1 defaulted to Ignore).
func isFailClosed(p *admissionregistrationv1.FailurePolicyType) bool {
	return p == nil || *p == admissionregistrationv1.Fail
}

// readyAddressCount sums addresses across every EndpointSlice for the given
// Service, counting only endpoints whose Ready condition is true or unset
// (unset defaults to ready per the EndpointSlice API).
func readyAddressCount(snap *k8s.Snapshot, namespace, service string) int {
	count := 0
	for _, eps := range snap.EndpointSlices {
		if eps.Namespace != namespace || eps.Labels["kubernetes.io/service-name"] != service {
			continue
		}
		for _, ep := range eps.Endpoints {
			if ep.Conditions.Ready == nil || *ep.Conditions.Ready {
				count += len(ep.Addresses)
			}
		}
	}
	return count
}

// failurePolicyLiteral renders the real, honest failurePolicy value —
// Fail, Ignore, or an unset field (which defaults to Fail) — rather than
// assuming Fail for any non-nil pointer. Safe to call for both fail-closed
// and fail-open webhooks; WH-001 and WH-002's own Fail-closed findings
// only ever construct this from a Fail-or-unset FailurePolicy in practice,
// but WH-002's Ignore-policy findings (added alongside this function's
// signature change) genuinely need the real value, not an assumed one.
func failurePolicyLiteral(p *admissionregistrationv1.FailurePolicyType) string {
	if p == nil {
		return "<unset> (defaults to Fail)"
	}
	return string(*p)
}

// breakGlassAction is the last-resort "the cluster is bricked by this
// webhook" fix: delete the whole webhook configuration. Shared by WH-001
// and WH-002 so both point at the identical, correctly-cased kubectl
// resource name.
func breakGlassAction(patchResource, name string) *findings.RemediationAction {
	return &findings.RemediationAction{
		Label: "Break-glass",
		Risky: true,
		Steps: []string{
			"Cluster writes are bricked by this webhook and no other option is restoring health in time.",
			"Delete the webhook configuration, then restore it once the backend is healthy again.",
		},
		Command: fmt.Sprintf("kubectl delete %s %s", patchResource, shellQuote(name)),
	}
}

type wh002Params struct {
	Kind, PatchResource   string
	ConfigName, ConfigUID string
	WebhookName           string
	WebhookIndex          int
	FailurePolicySet      bool
	FailurePolicy         *admissionregistrationv1.FailurePolicyType
	SvcNamespace, SvcName string
	GlobalBlocker         bool
}

func wh002Finding(p wh002Params, targetVersion string) findings.Finding {
	msg := fmt.Sprintf(
		"%s %q: webhook %q (index %d in .webhooks) is fail-closed and its backend service %s/%s has zero ready endpoints — matching API writes will be rejected",
		p.Kind, p.ConfigName, p.WebhookName, p.WebhookIndex, p.SvcNamespace, p.SvcName)

	// Keep the primary runbook recovery-first, and keep the inspect-only
	// commands visually and textually separate from the emergency patch —
	// terminal/markdown output only ever renders this flat string (not
	// RemediationDetail's SafeFix/Emergency split below), so this is the
	// only place a CLI/CI reader sees the distinction. The destructive
	// delete path is intentionally confined to the explicitly risky
	// BreakGlass action, never surfaced here.
	patchOp := "replace"
	if !p.FailurePolicySet {
		patchOp = "add"
	}
	remediation := fmt.Sprintf(`Step 1 — restore the webhook backend (read-only, safe to run any time):

kubectl get svc %s -n %s
kubectl get endpointslices -n %s -l kubernetes.io/service-name=%s
kubectl get deploy,pods -n %s

Step 2 — only if you need immediate relief and cannot wait for the backend to recover:

This TEMPORARILY REMOVES the webhook's protection. The "test" operation
guards against the array index having shifted since this scan ran — the
patch aborts instead of silently touching the wrong webhook block.

kubectl patch %s %s --type='json' -p='[{"op":"test","path":"/webhooks/%d/name","value":"%s"},{"op":"%s","path":"/webhooks/%d/failurePolicy","value":"Ignore"}]'

Revert failurePolicy to Fail immediately after the backend recovers.`,
		shellQuote(p.SvcName), shellQuote(p.SvcNamespace),
		shellQuote(p.SvcNamespace), shellQuote(p.SvcName), shellQuote(p.SvcNamespace),
		p.PatchResource, shellQuote(p.ConfigName), p.WebhookIndex, p.WebhookName, patchOp, p.WebhookIndex)

	ref := findings.LiveResource(p.Kind, findings.ScopeCluster, "", p.ConfigName, p.ConfigUID)
	return findings.Finding{
		RuleID:     "WH-002",
		Severity:   findings.SeverityBlocker,
		Confidence: findings.TierObserved,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		Evidence: []string{
			fmt.Sprintf("webhook name: %s", p.WebhookName),
			fmt.Sprintf("webhook index: %d", p.WebhookIndex),
			fmt.Sprintf("backend service: %s/%s", p.SvcNamespace, p.SvcName),
			"ready endpoint address count: 0",
			fmt.Sprintf("failurePolicy: %s", failurePolicyLiteral(p.FailurePolicy)),
		},
		Remediation:       remediation,
		RemediationDetail: wh002RemediationDetail(p),
		GlobalBlocker:     p.GlobalBlocker,
		// Keyed on parent config UID + webhook block name, not array index or
		// position: reordering .webhooks[] must not mint a new fingerprint
		// for an already-known failure, and two distinct failing webhook
		// blocks in the same config must not collide onto one fingerprint.
		Fingerprint: findings.FingerprintV2("WH-002", targetVersion, p.WebhookName, ref),
	}
}

func wh002RemediationDetail(p wh002Params) *findings.RemediationDetail {
	patchOp := "replace"
	if !p.FailurePolicySet {
		patchOp = "add"
	}
	return &findings.RemediationDetail{
		Changes: []findings.RemediationChange{
			{Field: "endpoint count", Current: "0", Required: ">= 1"},
		},
		SafeFix: &findings.RemediationAction{
			Label: "Safe fix",
			Steps: []string{
				"Restore the backend's health first — the webhook itself doesn't need to change once its Service has healthy endpoints again.",
			},
			Command: fmt.Sprintf(
				"kubectl get svc %s -n %s\nkubectl get endpointslices -n %s -l kubernetes.io/service-name=%s\nkubectl get deploy -n %s\nkubectl get pods -n %s --show-labels",
				shellQuote(p.SvcName), shellQuote(p.SvcNamespace), shellQuote(p.SvcNamespace), shellQuote(p.SvcName), shellQuote(p.SvcNamespace), shellQuote(p.SvcNamespace)),
		},
		Emergency: &findings.RemediationAction{
			Label: "Temporary mitigation",
			Risky: true,
			Steps: []string{
				"Only use for the duration of the incident — this removes the webhook's protection entirely.",
				"Revert (set failurePolicy back to Fail) once the backend is healthy again.",
			},
			// ConfigName is a plain positional resource-name argument, so it's
			// shellQuoted like everywhere else. WebhookName stays unquoted —
			// it's a JSON string value nested inside the single-quoted
			// -p='[...]' payload, where shellQuote's escaping would nest
			// incorrectly and break the patch.
			Command: fmt.Sprintf(`kubectl patch %s %s --type='json' -p='[{"op":"test","path":"/webhooks/%d/name","value":"%s"},{"op":"%s","path":"/webhooks/%d/failurePolicy","value":"Ignore"}]'`,
				p.PatchResource, shellQuote(p.ConfigName), p.WebhookIndex, p.WebhookName, patchOp, p.WebhookIndex),
		},
		BreakGlass:     breakGlassAction(p.PatchResource, p.ConfigName),
		VerifyCommand:  fmt.Sprintf("kubectl get endpointslices -n %s -l kubernetes.io/service-name=%s", shellQuote(p.SvcNamespace), shellQuote(p.SvcName)),
		ExpectedResult: "endpoint count >= 1",
	}
}
