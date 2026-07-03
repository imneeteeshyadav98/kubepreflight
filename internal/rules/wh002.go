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
// check WH-002).
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
			out = append(out, wh002Finding("ValidatingWebhookConfiguration", "validatingwebhookconfiguration",
				cfg.Name, string(cfg.UID), wh.Name, i, svc.Namespace, svc.Name, targetVersion))
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
			out = append(out, wh002Finding("MutatingWebhookConfiguration", "mutatingwebhookconfiguration",
				cfg.Name, string(cfg.UID), wh.Name, i, svc.Namespace, svc.Name, targetVersion))
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

func wh002Finding(kind, patchResource, name, uid, webhookName string, webhookIndex int, svcNamespace, svcName, targetVersion string) findings.Finding {
	msg := fmt.Sprintf(
		"%s %q: webhook %q (index %d in .webhooks) is fail-closed and its backend service %s/%s has zero ready endpoints — matching API writes will be rejected",
		kind, name, webhookName, webhookIndex, svcNamespace, svcName)

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
		svcNamespace, svcName,
		patchResource, name, webhookIndex,
		patchResource, name)

	ref := findings.LiveResource(kind, findings.ScopeCluster, "", name, uid)
	return findings.Finding{
		RuleID:     "WH-002",
		Severity:   findings.SeverityBlocker,
		Confidence: findings.TierStaticCertain,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		Evidence: []string{
			fmt.Sprintf("webhook name: %s", webhookName),
			fmt.Sprintf("webhook index: %d", webhookIndex),
			fmt.Sprintf("backend service: %s/%s", svcNamespace, svcName),
			"ready endpoint address count: 0",
		},
		Remediation: remediation,
		// Keyed on parent config UID + webhook block name, not array index or
		// position: reordering .webhooks[] must not mint a new fingerprint
		// for an already-known failure, and two distinct failing webhook
		// blocks in the same config must not collide onto one fingerprint.
		Fingerprint: findings.FingerprintV2("WH-002", targetVersion, webhookName, ref),
	}
}
