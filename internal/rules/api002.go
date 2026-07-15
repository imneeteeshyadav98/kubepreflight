package rules

import (
	"fmt"

	"kubepreflight/internal/collectors/k8s"
	"kubepreflight/internal/collectors/manifest"
	"kubepreflight/internal/findings"
)

// API002 flags deprecated Kubernetes APIs that are still served at the
// target version, but have a known future removal. API-001 owns the hard
// blocker case once target reaches RemovedInVersion; this rule is the
// earlier warning state that feeds the API compatibility scorecard's
// deprecated-API counts without changing upgrade exit-code semantics.
type API002 struct{}

func (API002) ID() string { return "API-002" }

func (API002) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	targetMajor, targetMinor, err := parseMajorMinor(targetVersion)
	if err != nil {
		return nil, fmt.Errorf("API-002: invalid target version %q: %w", targetVersion, err)
	}

	var out []findings.Finding
	if sc.K8s != nil {
		for _, obj := range sc.K8s.DeprecatedAPIUsage {
			decision, err := resolveAPIRemoval(obj.Group, obj.Version, obj.Kind)
			if err != nil {
				return nil, fmt.Errorf("API-002: %w", err)
			}
			if !targetBeforeRemoval(decision.RemovedInVersion, targetMajor, targetMinor) {
				continue
			}
			if isEphemeralEvent(obj.DeprecatedAPI) {
				continue
			}
			out = append(out, api002LiveFinding(obj, targetVersion, decision))
		}
	}
	if sc.Manifests != nil {
		for _, obj := range sc.Manifests.DeprecatedAPIUsage {
			decision, err := resolveAPIRemoval(obj.Group, obj.Version, obj.Kind)
			if err != nil {
				return nil, fmt.Errorf("API-002: %w", err)
			}
			if !targetBeforeRemoval(decision.RemovedInVersion, targetMajor, targetMinor) {
				continue
			}
			out = append(out, api002ManifestFinding(obj, targetVersion, decision))
		}
	}
	return mergeAPI001Findings(out), nil
}

func targetBeforeRemoval(removedInVersion string, targetMajor, targetMinor int) bool {
	removedMajor, removedMinor, err := parseMajorMinor(removedInVersion)
	if err != nil {
		return false
	}
	return targetMajor == removedMajor && targetMinor < removedMinor
}

func api002LiveFinding(obj k8s.DeprecatedAPIObject, targetVersion string, decision apiRemovalDecision) findings.Finding {
	gv := obj.Group + "/" + obj.Version
	resourceLabel := obj.Name
	if obj.Namespace != "" {
		resourceLabel = obj.Namespace + "/" + obj.Name
	}
	msg := fmt.Sprintf(
		"%s %q (apiVersion %s) uses a deprecated Kubernetes API that is still served at target %s but will be removed in Kubernetes %s",
		obj.Kind, resourceLabel, gv, targetVersion, decision.RemovedInVersion)
	remediation := fmt.Sprintf("Plan migration to %s before upgrading to Kubernetes %s or later. This API is still served at target %s, so this is a readiness warning rather than a hard upgrade blocker.",
		obj.Replacement, decision.RemovedInVersion, targetVersion)

	ref := findings.LiveResource(obj.Kind, apiResourceScope(obj.Namespaced), obj.Namespace, obj.Name, obj.UID)
	evidence := []string{
		fmt.Sprintf("apiVersion: %s", gv),
		fmt.Sprintf("removed in: Kubernetes %s", decision.RemovedInVersion),
		fmt.Sprintf("target version: %s", targetVersion),
		"status: deprecated but still served at target version",
		"detected via: live cluster object",
	}
	evidence = append(evidence, decision.evidence()...)
	return findings.Finding{
		RuleID:            "API-002",
		Severity:          findings.SeverityWarning,
		Confidence:        findings.TierStaticCertain,
		Message:           msg,
		Resources:         []findings.ResourceReference{ref},
		Evidence:          evidence,
		Remediation:       remediation,
		RemediationDetail: api001RemediationDetail(gv, obj.ReplacementAPIVersion, "", targetVersion),
		Fingerprint:       findings.FingerprintV2("API-002", targetVersion, "", ref),
	}
}

func api002ManifestFinding(obj manifest.DeprecatedAPIObject, targetVersion string, decision apiRemovalDecision) findings.Finding {
	gv := obj.Group + "/" + obj.Version
	resourceLabel := obj.Name
	if obj.Namespace != "" {
		resourceLabel = obj.Namespace + "/" + obj.Name
	}
	msg := fmt.Sprintf(
		"%s %q (apiVersion %s) in %s uses a deprecated Kubernetes API that is still served at target %s but will be removed in Kubernetes %s",
		obj.Kind, resourceLabel, gv, obj.SourcePath, targetVersion, decision.RemovedInVersion)
	remediation := fmt.Sprintf("Update this manifest to %s before upgrading to Kubernetes %s or later. It can still be applied at target %s, but leaving it in source control preserves a future upgrade risk.",
		obj.Replacement, decision.RemovedInVersion, targetVersion)

	ref := findings.ManifestResource(obj.Kind, apiResourceScope(obj.Namespaced), obj.Namespace, obj.Name, obj.SourcePath)
	evidence := []string{
		fmt.Sprintf("apiVersion: %s", gv),
		fmt.Sprintf("removed in: Kubernetes %s", decision.RemovedInVersion),
		fmt.Sprintf("target version: %s", targetVersion),
		"status: deprecated but still served at target version",
		fmt.Sprintf("source: %s", obj.SourcePath),
	}
	evidence = append(evidence, decision.evidence()...)
	return findings.Finding{
		RuleID:            "API-002",
		Severity:          findings.SeverityWarning,
		Confidence:        findings.TierStaticCertain,
		Message:           msg,
		Resources:         []findings.ResourceReference{ref},
		Evidence:          evidence,
		Remediation:       remediation,
		RemediationDetail: api001RemediationDetail(gv, obj.ReplacementAPIVersion, obj.SourcePath, targetVersion),
		Fingerprint:       findings.FingerprintV2("API-002", targetVersion, "", ref),
	}
}
