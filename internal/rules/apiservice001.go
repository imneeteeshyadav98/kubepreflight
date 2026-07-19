package rules

import (
	"fmt"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

type APIService001 struct{}

func (APIService001) ID() string { return "APISERVICE-001" }

func (APIService001) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	if sc.K8s == nil {
		return nil, nil
	}
	if _, unavailable := sc.K8s.Errors["apiservices"]; unavailable {
		return nil, nil
	}
	var out []findings.Finding
	for _, apiService := range sc.K8s.UnavailableAPIServices {
		ref := findings.LiveResource("APIService", findings.ScopeCluster, "", apiService.Name, apiService.UID)
		out = append(out, findings.Finding{
			RuleID: "APISERVICE-001", Severity: findings.SeverityBlocker, Confidence: findings.TierObserved,
			Message:     fmt.Sprintf("APIService %q is not Available — aggregated API discovery or requests can fail during upgrade validation and controller reconciliation", apiService.Name),
			Resources:   []findings.ResourceReference{ref},
			Evidence:    []string{fmt.Sprintf("reason: %s", apiService.Reason), fmt.Sprintf("message: %s", apiService.Message)},
			Remediation: "Restore the aggregated API backend and its APIService TLS/service configuration before upgrading.",
			RemediationDetail: &findings.RemediationDetail{
				Changes:       []findings.RemediationChange{{Field: "Available condition", Current: "False", Required: "True"}},
				SafeFix:       &findings.RemediationAction{Label: "Safe fix", Steps: []string{"Inspect the APIService condition and restore the backing service, endpoints, and CA/TLS configuration."}, Command: fmt.Sprintf("kubectl describe apiservice %s", shellQuote(apiService.Name))},
				VerifyCommand: fmt.Sprintf("kubectl get apiservice %s -o jsonpath='{.status.conditions[?(@.type==\"Available\")].status}'", shellQuote(apiService.Name)), ExpectedResult: "True",
			},
			Fingerprint: findings.FingerprintV2("APISERVICE-001", targetVersion, "", ref),
		})
	}
	return out, nil
}
