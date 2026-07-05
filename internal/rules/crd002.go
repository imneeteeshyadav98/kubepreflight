package rules

import (
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"kubepreflight/internal/findings"
)

// CRD002 flags a CRD conversion webhook whose in-cluster Service has no
// ready endpoints. API conversion can be required while reading or updating
// existing objects during an upgrade, so an unavailable fail-path is a hard
// readiness blocker.
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
		if conversion == nil || conversion.Strategy != apiextensionsv1.WebhookConverter || conversion.Webhook == nil || conversion.Webhook.ClientConfig == nil || conversion.Webhook.ClientConfig.Service == nil {
			continue
		}
		svc := conversion.Webhook.ClientConfig.Service
		if readyAddressCount(sc.K8s, svc.Namespace, svc.Name) > 0 {
			continue
		}
		ref := findings.LiveResource("CustomResourceDefinition", findings.ScopeCluster, "", crd.Name, string(crd.UID))
		out = append(out, findings.Finding{
			RuleID: "CRD-002", Severity: findings.SeverityBlocker, Confidence: findings.TierObserved,
			Message:     fmt.Sprintf("CustomResourceDefinition %q uses conversion webhook service %s/%s, which has zero ready endpoints — conversion requests can fail during reads, writes, and controller reconciliation", crd.Name, svc.Namespace, svc.Name),
			Resources:   []findings.ResourceReference{ref},
			Evidence:    []string{fmt.Sprintf("conversion strategy: %s", conversion.Strategy), fmt.Sprintf("service: %s/%s", svc.Namespace, svc.Name), "ready endpoint address count: 0"},
			Remediation: "Restore the conversion webhook backend before upgrading. Do not remove conversion configuration unless every stored object has been migrated and all served versions use compatible schemas.",
			RemediationDetail: &findings.RemediationDetail{
				Changes:       []findings.RemediationChange{{Field: "conversion webhook endpoints", Current: "0", Required: ">= 1"}},
				SafeFix:       &findings.RemediationAction{Label: "Safe fix", Steps: []string{"Restore the conversion webhook deployment/service backend; changing CRD conversion strategy is not a safe incident shortcut."}, Command: fmt.Sprintf("kubectl get svc %s -n %s\nkubectl get endpointslices -n %s -l kubernetes.io/service-name=%s", svc.Name, svc.Namespace, svc.Namespace, svc.Name)},
				VerifyCommand: fmt.Sprintf("kubectl get endpointslices -n %s -l kubernetes.io/service-name=%s", svc.Namespace, svc.Name), ExpectedResult: "endpoint count >= 1",
			},
			Fingerprint: findings.FingerprintV2("CRD-002", targetVersion, svc.Namespace+"/"+svc.Name, ref),
		})
	}
	return out, nil
}
