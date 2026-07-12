package rules

import (
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"kubepreflight/internal/findings"
)

// acceptedConversionReviewVersions mirrors apiextensions-apiserver's own
// accepted-version set (pkg/apis/apiextensions/validation/validation.go,
// AcceptedConversionReviewVersions) — v1 and v1beta1 are the only
// ConversionReview versions any API server has ever understood. If none of
// a CRD's conversionReviewVersions falls in this set, conversion fails
// outright: "If none of the versions specified in this list are supported
// by API server, conversion will fail for the custom resource."
var acceptedConversionReviewVersions = map[string]bool{"v1": true, "v1beta1": true}

// CRD002 flags a CRD with strategy: Webhook conversion that can't actually
// convert: a missing/incomplete webhook configuration, a referenced Service
// that doesn't exist or has no ready endpoints, or a conversionReviewVersions
// list the API server can't use at all. API conversion can be required
// while reading or updating existing objects during an upgrade, so any of
// these is a hard readiness blocker, not just the backend-down case this
// rule originally covered.
//
// The malformed-webhook-config cases (strategy Webhook with a nil Webhook
// field, a nil ClientConfig, neither Service nor URL set, or an invalid
// conversionReviewVersions list) are all rejected by the API server's own
// admission validation on write — so in a well-behaved cluster they should
// be rare or impossible to observe live. They're checked anyway because
// that validation isn't guaranteed identical across every server version
// skew this tool might scan, and a live object reflecting one of these
// states is unambiguously broken regardless of how it got that way.
type CRD002 struct{}

func (CRD002) ID() string { return "CRD-002" }

func (CRD002) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	if sc.K8s == nil {
		return nil, nil
	}
	if _, unavailable := sc.K8s.Errors["customresourcedefinitions"]; unavailable {
		return nil, nil
	}
	if _, unavailable := sc.K8s.Errors["endpointslices"]; unavailable {
		return nil, nil
	}
	var out []findings.Finding
	for _, crd := range sc.K8s.CustomResourceDefinitions {
		conversion := crd.Spec.Conversion
		if conversion == nil || conversion.Strategy != apiextensionsv1.WebhookConverter {
			continue
		}
		out = append(out, crd002Findings(sc, crd, conversion.Webhook, targetVersion)...)
	}
	return out, nil
}

func crd002Findings(sc *ScanContext, crd apiextensionsv1.CustomResourceDefinition, webhook *apiextensionsv1.WebhookConversion, targetVersion string) []findings.Finding {
	ref := findings.LiveResource("CustomResourceDefinition", findings.ScopeCluster, "", crd.Name, string(crd.UID))

	if webhook == nil {
		return []findings.Finding{crd002ConfigFinding(ref, crd, targetVersion, "no-webhook",
			fmt.Sprintf("CustomResourceDefinition %q sets conversion strategy Webhook but has no webhook configuration — conversion is required whenever strategy is Webhook", crd.Name),
			[]string{"conversion strategy: Webhook", "webhook: not set"},
			"Set spec.conversion.webhook with a valid clientConfig and conversionReviewVersions, or change the conversion strategy to None if no version conversion is actually needed.",
		)}
	}

	var out []findings.Finding

	if webhook.ClientConfig == nil {
		out = append(out, crd002ConfigFinding(ref, crd, targetVersion, "no-client-config",
			fmt.Sprintf("CustomResourceDefinition %q's conversion webhook has no clientConfig — the API server has no way to reach it", crd.Name),
			[]string{"conversion strategy: Webhook", "webhook.clientConfig: not set"},
			"Set spec.conversion.webhook.clientConfig with either a service or a url.",
		))
	} else if webhook.ClientConfig.Service == nil && webhook.ClientConfig.URL == nil {
		out = append(out, crd002ConfigFinding(ref, crd, targetVersion, "no-service-or-url",
			fmt.Sprintf("CustomResourceDefinition %q's conversion webhook clientConfig sets neither service nor url — exactly one is required", crd.Name),
			[]string{"conversion strategy: Webhook", "clientConfig.service: not set", "clientConfig.url: not set"},
			"Set exactly one of spec.conversion.webhook.clientConfig.service or .url.",
		))
	} else if svc := webhook.ClientConfig.Service; svc != nil {
		out = append(out, crd002ServiceFindings(sc, ref, crd, svc, targetVersion)...)
	}
	// A URL-based clientConfig is left unvalidated here beyond "is it set" —
	// this tool doesn't probe arbitrary external endpoints.

	if reviewFinding, ok := crd002ReviewVersionsFinding(ref, crd, webhook.ConversionReviewVersions, targetVersion); ok {
		out = append(out, reviewFinding)
	}

	return out
}

// crd002ServiceFindings preserves the original, already-shipped-and-tested
// "zero ready endpoints" check exactly, adding one case in front of it:
// a Service reference that doesn't resolve to any live Service object at
// all gets its own clearer finding instead of silently falling through to
// the same "0 ready endpoints" message a real-but-unhealthy Service would
// also produce.
func crd002ServiceFindings(sc *ScanContext, ref findings.ResourceReference, crd apiextensionsv1.CustomResourceDefinition, svc *apiextensionsv1.ServiceReference, targetVersion string) []findings.Finding {
	if !crd002ServiceExists(sc, svc.Namespace, svc.Name) {
		return []findings.Finding{crd002ConfigFinding(ref, crd, targetVersion, "service-not-found",
			fmt.Sprintf("CustomResourceDefinition %q uses conversion webhook service %s/%s, which does not exist in this cluster", crd.Name, svc.Namespace, svc.Name),
			[]string{fmt.Sprintf("conversion strategy: %s", apiextensionsv1.WebhookConverter), fmt.Sprintf("service: %s/%s", svc.Namespace, svc.Name), "service object: not found"},
			"Create the referenced Service and its backing workload, or point the conversion webhook at the correct service.",
		)}
	}

	if readyAddressCount(sc.K8s, svc.Namespace, svc.Name) > 0 {
		return nil
	}

	return []findings.Finding{{
		RuleID: "CRD-002", Severity: findings.SeverityBlocker, Confidence: findings.TierObserved,
		Message:     fmt.Sprintf("CustomResourceDefinition %q uses conversion webhook service %s/%s, which has zero ready endpoints — conversion requests can fail during reads, writes, and controller reconciliation", crd.Name, svc.Namespace, svc.Name),
		Resources:   []findings.ResourceReference{ref},
		Evidence:    []string{fmt.Sprintf("conversion strategy: %s", apiextensionsv1.WebhookConverter), fmt.Sprintf("service: %s/%s", svc.Namespace, svc.Name), "ready endpoint address count: 0"},
		Remediation: "Restore the conversion webhook backend before upgrading. Do not remove conversion configuration unless every stored object has been migrated and all served versions use compatible schemas.",
		RemediationDetail: &findings.RemediationDetail{
			Changes:       []findings.RemediationChange{{Field: "conversion webhook endpoints", Current: "0", Required: ">= 1"}},
			SafeFix:       &findings.RemediationAction{Label: "Safe fix", Steps: []string{"Restore the conversion webhook deployment/service backend; changing CRD conversion strategy is not a safe incident shortcut."}, Command: fmt.Sprintf("kubectl get svc %s -n %s\nkubectl get endpointslices -n %s -l kubernetes.io/service-name=%s", shellQuote(svc.Name), shellQuote(svc.Namespace), shellQuote(svc.Namespace), shellQuote(svc.Name))},
			VerifyCommand: fmt.Sprintf("kubectl get endpointslices -n %s -l kubernetes.io/service-name=%s", shellQuote(svc.Namespace), shellQuote(svc.Name)), ExpectedResult: "endpoint count >= 1",
		},
		Fingerprint: findings.FingerprintV2("CRD-002", targetVersion, svc.Namespace+"/"+svc.Name, ref),
	}}
}

func crd002ServiceExists(sc *ScanContext, namespace, name string) bool {
	for _, svc := range sc.K8s.Services {
		if svc.Namespace == namespace && svc.Name == name {
			return true
		}
	}
	return false
}

// crd002ReviewVersionsFinding checks conversionReviewVersions independently
// of clientConfig/Service health -- a webhook with a perfectly healthy
// backend still can't convert if the API server can't use any version in
// this list.
func crd002ReviewVersionsFinding(ref findings.ResourceReference, crd apiextensionsv1.CustomResourceDefinition, versions []string, targetVersion string) (findings.Finding, bool) {
	if len(versions) == 0 {
		return crd002ConfigFinding(ref, crd, targetVersion, "empty-review-versions",
			fmt.Sprintf("CustomResourceDefinition %q's conversion webhook has an empty conversionReviewVersions list — the API server has no version it can use to call the webhook, so conversion will fail for this resource", crd.Name),
			[]string{"conversion strategy: Webhook", "conversionReviewVersions: (empty)"},
			"Set spec.conversion.webhook.conversionReviewVersions to include at least one version this API server understands, e.g. [\"v1\", \"v1beta1\"].",
		), true
	}
	for _, v := range versions {
		if acceptedConversionReviewVersions[v] {
			return findings.Finding{}, false
		}
	}
	return crd002ConfigFinding(ref, crd, targetVersion, "unsupported-review-versions",
		fmt.Sprintf("CustomResourceDefinition %q's conversion webhook lists conversionReviewVersions %v, none of which this API server accepts — conversion will fail for this resource", crd.Name, versions),
		[]string{"conversion strategy: Webhook", fmt.Sprintf("conversionReviewVersions: %v", versions), "accepted by this API server: [v1, v1beta1]"},
		"Add \"v1\" (or \"v1beta1\" for older clusters) to spec.conversion.webhook.conversionReviewVersions.",
	), true
}

// crd002ConfigFinding builds a Blocker finding for a static conversion-
// webhook misconfiguration (as opposed to the live endpoint-health check,
// which carries its own richer RemediationDetail with a verify command).
// discriminator keeps each distinct problem's fingerprint from colliding
// with another problem on the same CRD, since a single CRD can fail more
// than one of these checks at once.
func crd002ConfigFinding(ref findings.ResourceReference, crd apiextensionsv1.CustomResourceDefinition, targetVersion, discriminator, message string, evidence []string, remediation string) findings.Finding {
	return findings.Finding{
		RuleID:      "CRD-002",
		Severity:    findings.SeverityBlocker,
		Confidence:  findings.TierStaticCertain,
		Message:     message,
		Resources:   []findings.ResourceReference{ref},
		Evidence:    evidence,
		Remediation: remediation,
		Fingerprint: findings.FingerprintV2("CRD-002", targetVersion, crd.Name+":"+discriminator, ref),
	}
}
