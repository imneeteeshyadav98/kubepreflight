package rules

import (
	"fmt"
	"strings"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

// CRD001 flags CRDs that still have objects persisted in a non-storage
// version. A still-served legacy stored version is a migration warning; a
// stored version that is no longer served by the CRD is a blocker because
// those objects may require unavailable conversion paths during upgrade work.
type CRD001 struct{}

func (CRD001) ID() string { return "CRD-001" }

func (CRD001) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	if sc.K8s == nil {
		return nil, nil
	}
	if _, unavailable := sc.K8s.Errors["customresourcedefinitions"]; unavailable {
		return nil, nil
	}
	var out []findings.Finding
	for _, crd := range sc.K8s.CustomResourceDefinitions {
		storage := ""
		servedVersions := map[string]bool{}
		for _, version := range crd.Spec.Versions {
			servedVersions[version.Name] = version.Served
			if version.Storage {
				storage = version.Name
			}
		}
		var legacy []string
		var unavailable []string
		for _, stored := range crd.Status.StoredVersions {
			if stored != storage {
				legacy = append(legacy, stored)
			}
			if served, ok := servedVersions[stored]; !ok || !served {
				unavailable = append(unavailable, stored)
			}
		}
		if len(legacy) == 0 {
			continue
		}
		severity := findings.SeverityWarning
		message := fmt.Sprintf("CustomResourceDefinition %q still stores objects in legacy version(s) %s while %s is the storage version — migrate stored objects before removing those versions", crd.Name, strings.Join(legacy, ", "), storage)
		remediation := "Rewrite all custom resources through the current storage version, confirm status.storedVersions contains only that version, then remove obsolete served versions from the CRD."
		evidence := []string{fmt.Sprintf("status.storedVersions: %s", strings.Join(crd.Status.StoredVersions, ", ")), fmt.Sprintf("current storage version: %s", storage)}
		if len(unavailable) > 0 {
			severity = findings.SeverityBlocker
			message = fmt.Sprintf("CustomResourceDefinition %q still stores objects in unavailable version(s) %s while %s is the storage version — migrate stored objects before upgrade or CRD conversion paths can fail", crd.Name, strings.Join(unavailable, ", "), storage)
			remediation = "Restore or serve the stored version long enough to migrate all custom resources through the current storage version, then confirm status.storedVersions contains only that version."
			evidence = append(evidence, fmt.Sprintf("unavailable stored version(s): %s", strings.Join(unavailable, ", ")))
		}
		ref := findings.LiveResource("CustomResourceDefinition", findings.ScopeCluster, "", crd.Name, string(crd.UID))
		out = append(out, findings.Finding{
			RuleID: "CRD-001", Severity: severity, Confidence: findings.TierStaticCertain,
			Message:     message,
			Resources:   []findings.ResourceReference{ref},
			Evidence:    evidence,
			Remediation: remediation,
			RemediationDetail: &findings.RemediationDetail{
				Changes:       []findings.RemediationChange{{Field: "status.storedVersions", Current: strings.Join(crd.Status.StoredVersions, ", "), Required: storage}},
				SafeFix:       &findings.RemediationAction{Label: "Safe fix", Steps: []string{"Back up the custom resources and follow the Kubernetes storage-version migration procedure; do not edit CRD status by hand before objects are rewritten."}, Command: fmt.Sprintf("kubectl get crd %s -o yaml", shellQuote(crd.Name))},
				VerifyCommand: fmt.Sprintf("kubectl get crd %s -o jsonpath='{.status.storedVersions}'", shellQuote(crd.Name)), ExpectedResult: storage,
			},
			Fingerprint: findings.FingerprintV2("CRD-001", targetVersion, strings.Join(legacy, ","), ref),
		})
	}
	return out, nil
}
