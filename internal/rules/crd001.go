package rules

import (
	"fmt"
	"strings"

	"kubepreflight/internal/findings"
)

// CRD001 warns when a CRD still has objects persisted in a non-storage
// version. That state is supported, but it becomes an upgrade/migration
// blocker when the old version is removed from the CRD.
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
		for _, version := range crd.Spec.Versions {
			if version.Storage {
				storage = version.Name
				break
			}
		}
		var legacy []string
		for _, stored := range crd.Status.StoredVersions {
			if stored != storage {
				legacy = append(legacy, stored)
			}
		}
		if len(legacy) == 0 {
			continue
		}
		ref := findings.LiveResource("CustomResourceDefinition", findings.ScopeCluster, "", crd.Name, string(crd.UID))
		out = append(out, findings.Finding{
			RuleID: "CRD-001", Severity: findings.SeverityWarning, Confidence: findings.TierStaticCertain,
			Message:     fmt.Sprintf("CustomResourceDefinition %q still stores objects in legacy version(s) %s while %s is the storage version — migrate stored objects before removing those versions", crd.Name, strings.Join(legacy, ", "), storage),
			Resources:   []findings.ResourceReference{ref},
			Evidence:    []string{fmt.Sprintf("status.storedVersions: %s", strings.Join(crd.Status.StoredVersions, ", ")), fmt.Sprintf("current storage version: %s", storage)},
			Remediation: "Rewrite all custom resources through the current storage version, confirm status.storedVersions contains only that version, then remove obsolete served versions from the CRD.",
			RemediationDetail: &findings.RemediationDetail{
				Changes:       []findings.RemediationChange{{Field: "status.storedVersions", Current: strings.Join(crd.Status.StoredVersions, ", "), Required: storage}},
				SafeFix:       &findings.RemediationAction{Label: "Safe fix", Steps: []string{"Back up the custom resources and follow the Kubernetes storage-version migration procedure; do not edit CRD status by hand before objects are rewritten."}, Command: fmt.Sprintf("kubectl get crd %s -o yaml", crd.Name)},
				VerifyCommand: fmt.Sprintf("kubectl get crd %s -o jsonpath='{.status.storedVersions}'", crd.Name), ExpectedResult: storage,
			},
			Fingerprint: findings.FingerprintV2("CRD-001", targetVersion, strings.Join(legacy, ","), ref),
		})
	}
	return out, nil
}
