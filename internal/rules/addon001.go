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

// ADDON002 flags high-impact EKS add-ons whose target-version compatibility
// could not be verified. ADDON-001 owns known incompatibility; this rule owns
// the "unknown but important" state so it does not disappear into inventory.
type ADDON002 struct{}

func (ADDON002) ID() string { return "ADDON-002" }

func (ADDON002) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	if sc.AWS == nil {
		return nil, nil
	}
	var out []findings.Finding
	for _, addon := range sc.AWS.Addons {
		if !isPR1CriticalAddon(addon.Name) {
			continue
		}
		err, unavailable := sc.AWS.Errors["describe-addon-versions:"+addon.Name]
		if !unavailable {
			continue
		}
		out = append(out, addon002Finding(addon, targetVersion, err))
	}
	return out, nil
}

func isPR1CriticalAddon(name string) bool {
	switch name {
	case "vpc-cni", "kube-proxy":
		return true
	default:
		return false
	}
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
	remediation += "Upgrade order: validate CNI first, then kube-proxy, then DNS/storage/other add-ons. "
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
			fmt.Sprintf("installed add-on: %s", addon.Name),
			fmt.Sprintf("current version: %s", addon.CurrentVersion),
			fmt.Sprintf("target Kubernetes version: %s", targetVersion),
			fmt.Sprintf("minimum supported version: %s", required),
			fmt.Sprintf("AWS-reported compatible versions: %s", strings.Join(addon.CompatibleVersions, ", ")),
			fmt.Sprintf("recommended upgrade version: %s", required),
			fmt.Sprintf("required upgrade order: %s", addonUpgradeOrder(addon.Name)),
			"compatibility status: incompatible",
			"confidence/source: AWS EKS DescribeAddonVersions",
		},
		Remediation:       remediation,
		RemediationDetail: detail,
		Fingerprint:       findings.FingerprintV2("ADDON-001", targetVersion, "", ref),
	}
}

func addon002Finding(addon awscol.AddonRecord, targetVersion string, err error) findings.Finding {
	ref := findings.AWSResource("EKSAddon", addon.Name, addon.Name)
	msg := fmt.Sprintf(
		"EKS add-on %q version %s could not be verified against target Kubernetes %s — confirm compatibility before starting the upgrade",
		addon.Name, addon.CurrentVersion, targetVersion)
	detail := &findings.RemediationDetail{
		Changes: []findings.RemediationChange{{Field: "add-on compatibility", Current: "unknown", Required: "verified compatible with target Kubernetes " + targetVersion}},
		SafeFix: &findings.RemediationAction{Label: "Safe fix", Steps: []string{"Restore DescribeAddonVersions access or manually verify the add-on version against the EKS compatibility matrix before upgrading."}, Command: fmt.Sprintf("aws eks describe-addon-versions --addon-name %s --kubernetes-version %s", shellQuote(addon.Name), shellQuote(targetVersion))},
	}
	if addon.ClusterName != "" {
		detail.VerifyCommand = fmt.Sprintf("aws eks describe-addon --cluster-name %s --addon-name %s", shellQuote(addon.ClusterName), shellQuote(addon.Name))
	}
	return findings.Finding{
		RuleID:     "ADDON-002",
		Severity:   findings.SeverityWarning,
		Confidence: findings.TierProviderReported,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		Evidence: []string{
			fmt.Sprintf("installed add-on: %s", addon.Name),
			fmt.Sprintf("current version: %s", addon.CurrentVersion),
			fmt.Sprintf("target Kubernetes version: %s", targetVersion),
			"minimum supported version: unknown",
			"compatibility status: unknown",
			"confidence/source: AWS EKS DescribeAddonVersions unavailable",
			fmt.Sprintf("verification error: %v", err),
			fmt.Sprintf("recommended upgrade version: verify with AWS before upgrading %s", addon.Name),
			fmt.Sprintf("required upgrade order: %s", addonUpgradeOrder(addon.Name)),
		},
		Remediation:       "Verify the add-on against AWS's target-version compatibility data before upgrading. Treat VPC CNI and kube-proxy as early-order add-ons because networking and service proxy behavior underpin the rest of the cluster.",
		RemediationDetail: detail,
		Fingerprint:       findings.FingerprintV2("ADDON-002", targetVersion, "", ref),
	}
}

func addonUpgradeOrder(name string) string {
	switch name {
	case "vpc-cni":
		return "1. Amazon VPC CNI before kube-proxy and DNS/storage add-ons"
	case "kube-proxy":
		return "2. kube-proxy after VPC CNI and before CoreDNS/storage add-ons"
	default:
		return "review provider-recommended order"
	}
}
