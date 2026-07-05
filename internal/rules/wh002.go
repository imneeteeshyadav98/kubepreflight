package rules

import (
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"

	"kubepreflight/internal/collectors/k8s"
	"kubepreflight/internal/findings"
)

// WH002 flags a fail-closed admission webhook whose backend Service has
// zero ready endpoint addresses: every API write matching that webhook's
// rules will be rejected until the backend recovers (deep dive Section 5,
// check WH-002). When the same webhook also has catch-all scope (WH-001's
// condition), it's flagged GlobalBlocker — its outage doesn't just fail
// its own writes, it can fail kubectl/Helm remediation for anything else
// in the cluster too.
type WH002 struct{}

func (WH002) ID() string { return "WH-002" }

func (WH002) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	snap := sc.K8s
	var out []findings.Finding

	for _, cfg := range snap.ValidatingWebhookConfigs {
		for i, wh := range cfg.Webhooks {
			if !isFailClosed(wh.FailurePolicy) || wh.ClientConfig.Service == nil {
				continue
			}
			svc := wh.ClientConfig.Service
			if readyAddressCount(snap, svc.Namespace, svc.Name) > 0 {
				continue
			}
			catchAll, _ := hasCatchAllRule(wh.Rules)
			out = append(out, wh002Finding(wh002Params{
				Kind: "ValidatingWebhookConfiguration", PatchResource: "validatingwebhookconfiguration",
				ConfigName: cfg.Name, ConfigUID: string(cfg.UID),
				WebhookName: wh.Name, WebhookIndex: i,
				FailurePolicySet: wh.FailurePolicy != nil,
				SvcNamespace:     svc.Namespace, SvcName: svc.Name,
				GlobalBlocker: catchAll,
			}, targetVersion))
		}
	}

	for _, cfg := range snap.MutatingWebhookConfigs {
		for i, wh := range cfg.Webhooks {
			if !isFailClosed(wh.FailurePolicy) || wh.ClientConfig.Service == nil {
				continue
			}
			svc := wh.ClientConfig.Service
			if readyAddressCount(snap, svc.Namespace, svc.Name) > 0 {
				continue
			}
			catchAll, _ := hasCatchAllRule(wh.Rules)
			out = append(out, wh002Finding(wh002Params{
				Kind: "MutatingWebhookConfiguration", PatchResource: "mutatingwebhookconfiguration",
				ConfigName: cfg.Name, ConfigUID: string(cfg.UID),
				WebhookName: wh.Name, WebhookIndex: i,
				FailurePolicySet: wh.FailurePolicy != nil,
				SvcNamespace:     svc.Namespace, SvcName: svc.Name,
				GlobalBlocker: catchAll,
			}, targetVersion))
		}
	}

	return out, nil
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
// distinguishing an explicit "Fail" from an unset field that defaults to
// Fail, rather than collapsing both into one static string.
func failurePolicyLiteral(set bool) string {
	if set {
		return "Fail"
	}
	return "<unset> (defaults to Fail)"
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
		Command: fmt.Sprintf("kubectl delete %s %s", patchResource, name),
	}
}

type wh002Params struct {
	Kind, PatchResource   string
	ConfigName, ConfigUID string
	WebhookName           string
	WebhookIndex          int
	FailurePolicySet      bool
	SvcNamespace, SvcName string
	GlobalBlocker         bool
}

func wh002Finding(p wh002Params, targetVersion string) findings.Finding {
	msg := fmt.Sprintf(
		"%s %q: webhook %q (index %d in .webhooks) is fail-closed and its backend service %s/%s has zero ready endpoints — matching API writes will be rejected",
		p.Kind, p.ConfigName, p.WebhookName, p.WebhookIndex, p.SvcNamespace, p.SvcName)

	// Runbook lifted from deep dive Section 5.5, with <name>/<ns>/<svc>
	// filled in from this finding's own evidence.
	remediation := fmt.Sprintf(`Narrow scope or fail-open temporarily, then restore backend health:

# Inventory
kubectl get validatingwebhookconfigurations,mutatingwebhookconfigurations -o wide

# Check backend health for this webhook's service
kubectl get endpointslices -n %s -l kubernetes.io/service-name=%s

# Mitigate (temporary): narrow scope or fail-open
kubectl patch %s %s --type='json' \
  -p='[{"op":"replace","path":"/webhooks/%d/failurePolicy","value":"Ignore"}]'

# Break-glass (cluster is bricked by the webhook): delete the config
kubectl delete %s %s   # restore after recovery`,
		p.SvcNamespace, p.SvcName,
		p.PatchResource, p.ConfigName, p.WebhookIndex,
		p.PatchResource, p.ConfigName)

	ref := findings.LiveResource(p.Kind, findings.ScopeCluster, "", p.ConfigName, p.ConfigUID)
	return findings.Finding{
		RuleID:     "WH-002",
		Severity:   findings.SeverityBlocker,
		Confidence: findings.TierStaticCertain,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		Evidence: []string{
			fmt.Sprintf("webhook name: %s", p.WebhookName),
			fmt.Sprintf("webhook index: %d", p.WebhookIndex),
			fmt.Sprintf("backend service: %s/%s", p.SvcNamespace, p.SvcName),
			"ready endpoint address count: 0",
			fmt.Sprintf("failurePolicy: %s", failurePolicyLiteral(p.FailurePolicySet)),
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
				p.SvcName, p.SvcNamespace, p.SvcNamespace, p.SvcName, p.SvcNamespace, p.SvcNamespace),
		},
		Emergency: &findings.RemediationAction{
			Label: "Temporary mitigation",
			Risky: true,
			Steps: []string{
				"Only use for the duration of the incident — this removes the webhook's protection entirely.",
				"Revert (set failurePolicy back to Fail) once the backend is healthy again.",
			},
			Command: fmt.Sprintf(`kubectl patch %s %s --type='json' -p='[{"op":"%s","path":"/webhooks/%d/failurePolicy","value":"Ignore"}]'`,
				p.PatchResource, p.ConfigName, patchOp, p.WebhookIndex),
		},
		BreakGlass:     breakGlassAction(p.PatchResource, p.ConfigName),
		VerifyCommand:  fmt.Sprintf("kubectl get endpointslices -n %s -l kubernetes.io/service-name=%s", p.SvcNamespace, p.SvcName),
		ExpectedResult: "endpoint count >= 1",
	}
}
