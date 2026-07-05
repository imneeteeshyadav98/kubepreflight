package rules

import (
	"fmt"
	"strings"

	awscol "kubepreflight/internal/collectors/aws"
	"kubepreflight/internal/findings"
)

// ADDON001 flags an installed EKS add-on whose currently-installed version
// is not among AWS's own reported set of versions compatible with the
// scan's target Kubernetes version — a deterministic preflight check
// queryable before the upgrade even starts (deep dive Section 9, check
// ADDON-001).
type ADDON001 struct{}

func (ADDON001) ID() string { return "ADDON-001" }

func (ADDON001) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	if sc.AWS == nil {
		return nil, nil // AWS enrichment wasn't attempted or was gracefully skipped.
	}

	var out []findings.Finding
	for _, addon := range sc.AWS.Addons {
		if _, unavailable := sc.AWS.Errors["describe-addon-versions:"+addon.Name]; unavailable {
			continue
		}
		if isVersionCompatible(addon.CurrentVersion, addon.CompatibleVersions) {
			continue
		}
		out = append(out, addon001Finding(addon, targetVersion))
	}
	return out, nil
}

func isVersionCompatible(current string, compatible []string) bool {
	for _, v := range compatible {
		if v == current {
			return true
		}
	}
	return false
}

func addon001Finding(addon awscol.AddonRecord, targetVersion string) findings.Finding {
	var msg string
	if len(addon.CompatibleVersions) == 0 {
		msg = fmt.Sprintf(
			"EKS add-on %q version %s: AWS reports no compatible version of this add-on for target Kubernetes %s — it must be upgraded, replaced, or removed before upgrading",
			addon.Name, addon.CurrentVersion, targetVersion)
	} else {
		msg = fmt.Sprintf(
			"EKS add-on %q is on version %s, which is not in AWS's list of versions compatible with target Kubernetes %s",
			addon.Name, addon.CurrentVersion, targetVersion)
	}

	remediation := "Choose an AWS-reported compatible add-on version, review the add-on's current customizations, and update it in the provider-recommended sequence. "
	if len(addon.CompatibleVersions) > 0 {
		remediation += fmt.Sprintf("Compatible versions for target %s: %s. ", targetVersion, strings.Join(addon.CompatibleVersions, ", "))
	}
	remediation += "Confirm which fields are customized before choosing --resolve-conflicts: OVERWRITE silently destroys customizations, " +
		"PRESERVE keeps them but can fail the update, NONE fails on any conflict."

	ref := findings.AWSResource("EKSAddon", addon.Name, addon.Name)
	required := "a version compatible with the target"
	if len(addon.CompatibleVersions) > 0 {
		required = strings.Join(addon.CompatibleVersions, " or ")
	}
	detail := &findings.RemediationDetail{
		Changes: []findings.RemediationChange{{Field: "add-on version", Current: addon.CurrentVersion, Required: required}},
		SafeFix: &findings.RemediationAction{Label: "Safe fix", Steps: []string{"Review add-on customizations and choose a compatible version before updating; PRESERVE can fail on conflicts while OVERWRITE can destroy customizations."}, Command: fmt.Sprintf("aws eks describe-addon-versions --addon-name %s --kubernetes-version %s", shellQuote(addon.Name), shellQuote(targetVersion))},
	}
	if addon.ClusterName != "" {
		detail.VerifyCommand = fmt.Sprintf("aws eks describe-addon --cluster-name %s --addon-name %s", shellQuote(addon.ClusterName), shellQuote(addon.Name))
	}
	return findings.Finding{
		RuleID:     "ADDON-001",
		Severity:   findings.SeverityBlocker,
		Confidence: findings.TierProviderReported,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		Evidence: []string{
			fmt.Sprintf("current version: %s", addon.CurrentVersion),
			fmt.Sprintf("target Kubernetes version: %s", targetVersion),
			fmt.Sprintf("AWS-reported compatible versions: %s", strings.Join(addon.CompatibleVersions, ", ")),
		},
		Remediation:       remediation,
		RemediationDetail: detail,
		Fingerprint:       findings.FingerprintV2("ADDON-001", targetVersion, "", ref),
	}
}
