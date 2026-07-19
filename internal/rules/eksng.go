package rules

import (
	"fmt"
	"strings"

	awscol "github.com/imneeteeshyadav98/kubepreflight/internal/collectors/aws"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

// EKSNG001 flags AWS-reported health issues on an EKS managed node group.
type EKSNG001 struct{}

func (EKSNG001) ID() string { return "EKS-NG-001" }

func (EKSNG001) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	if sc.AWS == nil {
		return nil, nil
	}
	var out []findings.Finding
	for _, ng := range sc.AWS.Nodegroups {
		if len(ng.HealthIssues) == 0 {
			continue
		}
		out = append(out, eksNG001Finding(ng))
	}
	return out, nil
}

// EKSNG002 flags managed node groups with little rolling-update headroom.
type EKSNG002 struct{}

func (EKSNG002) ID() string { return "EKS-NG-002" }

func (EKSNG002) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	if sc.AWS == nil {
		return nil, nil
	}
	var out []findings.Finding
	for _, ng := range sc.AWS.Nodegroups {
		if ng.DesiredSize == nil || ng.MinSize == nil || *ng.DesiredSize > *ng.MinSize {
			continue
		}
		out = append(out, eksNG002Finding(ng))
	}
	return out, nil
}

// EKSNG003 surfaces launch-template/custom-AMI node groups for manual validation.
type EKSNG003 struct{}

func (EKSNG003) ID() string { return "EKS-NG-003" }

func (EKSNG003) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	if sc.AWS == nil {
		return nil, nil
	}
	var out []findings.Finding
	for _, ng := range sc.AWS.Nodegroups {
		if !ng.LaunchTemplate && ng.AMIType != "CUSTOM" {
			continue
		}
		out = append(out, eksNG003Finding(ng))
	}
	return out, nil
}

// EKSNG004 provides provider-inventory version context without duplicating
// NODE-001's live kubelet-skew blocker.
type EKSNG004 struct{}

func (EKSNG004) ID() string { return "EKS-NG-004" }

func (EKSNG004) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	if sc.AWS == nil {
		return nil, nil
	}
	var out []findings.Finding
	for _, ng := range sc.AWS.Nodegroups {
		if ng.Version == "" {
			continue
		}
		if path, _, ok := findings.UpgradePath(ng.Version, targetVersion); ok && len(path) > 1 {
			out = append(out, eksNG004Finding(ng, targetVersion))
		}
	}
	return out, nil
}

func eksNG001Finding(ng awscol.NodegroupRecord) findings.Finding {
	ref := eksNodegroupResource(ng)
	codes := make([]string, 0, len(ng.HealthIssues))
	evidence := []string{}
	for _, issue := range ng.HealthIssues {
		codes = append(codes, issue.Code)
		line := "health issue"
		if issue.Code != "" {
			line += " code: " + issue.Code
		}
		if issue.Message != "" {
			line += "; message: " + issue.Message
		}
		if len(issue.ResourceIDs) > 0 {
			line += "; resourceIds: " + strings.Join(issue.ResourceIDs, ", ")
		}
		evidence = append(evidence, line)
	}
	return findings.Finding{
		RuleID:      "EKS-NG-001",
		Severity:    findings.SeverityWarning,
		Confidence:  findings.TierProviderReported,
		Message:     fmt.Sprintf("Managed node group %q reports EKS health issue(s): %s. Review and resolve node group health before upgrade.", ng.Name, strings.Join(codes, ", ")),
		Resources:   []findings.ResourceReference{ref},
		Evidence:    evidence,
		Remediation: "Review the EKS managed node group health issue details, resolve the underlying AWS/node condition, and rerun KubePreflight before the upgrade window.",
		RemediationDetail: &findings.RemediationDetail{
			SafeFix:       &findings.RemediationAction{Label: "Inspect node group health", Steps: []string{"Inspect AWS-reported managed node group health and resolve any listed issues before upgrading."}, Command: describeNodegroupCommand(ng)},
			VerifyCommand: describeNodegroupCommand(ng),
		},
		Fingerprint: findings.FingerprintV2("EKS-NG-001", "", "", ref),
	}
}

func eksNG002Finding(ng awscol.NodegroupRecord) findings.Finding {
	ref := eksNodegroupResource(ng)
	evidence := []string{
		fmt.Sprintf("desiredSize: %s", int32PtrString(ng.DesiredSize)),
		fmt.Sprintf("minSize: %s", int32PtrString(ng.MinSize)),
		fmt.Sprintf("maxSize: %s", int32PtrString(ng.MaxSize)),
	}
	return findings.Finding{
		RuleID:      "EKS-NG-002",
		Severity:    findings.SeverityWarning,
		Confidence:  findings.TierProviderReported,
		Message:     fmt.Sprintf("Managed node group %q desired size equals or is below minimum size. Rolling update may have limited disruption headroom.", ng.Name),
		Resources:   []findings.ResourceReference{ref},
		Evidence:    evidence,
		Remediation: "Review node group capacity and disruption budgets before upgrade. Consider temporarily increasing desired capacity or otherwise creating eviction headroom for the change window.",
		RemediationDetail: &findings.RemediationDetail{
			Changes:       []findings.RemediationChange{{Field: "desired/min/max", Current: fmt.Sprintf("%s/%s/%s", int32PtrString(ng.DesiredSize), int32PtrString(ng.MinSize), int32PtrString(ng.MaxSize)), Required: "extra rolling-update headroom"}},
			SafeFix:       &findings.RemediationAction{Label: "Inspect scaling config", Steps: []string{"Inspect managed node group scaling and update settings before choosing any capacity change."}, Command: describeNodegroupCommand(ng)},
			VerifyCommand: describeNodegroupCommand(ng),
		},
		Fingerprint: findings.FingerprintV2("EKS-NG-002", "", "", ref),
	}
}

func eksNG003Finding(ng awscol.NodegroupRecord) findings.Finding {
	ref := eksNodegroupResource(ng)
	evidence := []string{
		fmt.Sprintf("launchTemplate: %t", ng.LaunchTemplate),
		fmt.Sprintf("amiType: %s", emptyDash(ng.AMIType)),
	}
	return findings.Finding{
		RuleID:      "EKS-NG-003",
		Severity:    findings.SeverityInfo,
		Confidence:  findings.TierProviderReported,
		Message:     fmt.Sprintf("Managed node group %q uses a launch template/custom AMI. Validate AMI, bootstrap, kubelet, and launch template upgrade path manually.", ng.Name),
		Resources:   []findings.ResourceReference{ref},
		Evidence:    evidence,
		Remediation: "Manually validate the launch template or custom AMI upgrade path, including bootstrap configuration, kubelet version, user data, and AMI release process.",
		RemediationDetail: &findings.RemediationDetail{
			SafeFix:       &findings.RemediationAction{Label: "Inspect node group configuration", Steps: []string{"Review the launch template/custom AMI before updating the managed node group version."}, Command: describeNodegroupCommand(ng)},
			VerifyCommand: describeNodegroupCommand(ng),
		},
		Fingerprint: findings.FingerprintV2("EKS-NG-003", "", "", ref),
	}
}

func eksNG004Finding(ng awscol.NodegroupRecord, targetVersion string) findings.Finding {
	ref := eksNodegroupResource(ng)
	return findings.Finding{
		RuleID:     "EKS-NG-004",
		Severity:   findings.SeverityInfo,
		Confidence: findings.TierProviderReported,
		Message:    fmt.Sprintf("Managed node group %q reports Kubernetes version %s while target is %s. Node kubelet skew is evaluated separately by NODE-001.", ng.Name, ng.Version, targetVersion),
		Resources:  []findings.ResourceReference{ref},
		Evidence: []string{
			fmt.Sprintf("node group Kubernetes version: %s", ng.Version),
			fmt.Sprintf("target Kubernetes version: %s", targetVersion),
			"NODE-001 evaluates actual Kubernetes node/kubelet skew separately.",
		},
		Remediation:       "Use this as provider inventory context. Confirm actual node kubelet skew in NODE-001 findings and update managed node groups in the provider-recommended sequence.",
		RemediationDetail: &findings.RemediationDetail{VerifyCommand: describeNodegroupCommand(ng)},
		Fingerprint:       findings.FingerprintV2("EKS-NG-004", targetVersion, "", ref),
	}
}

func eksNodegroupResource(ng awscol.NodegroupRecord) findings.ResourceReference {
	providerID := ng.Name
	if ng.ClusterName != "" {
		providerID = ng.ClusterName + "/" + ng.Name
	}
	return findings.AWSResource("EKSNodegroup", ng.Name, providerID)
}

func describeNodegroupCommand(ng awscol.NodegroupRecord) string {
	if ng.ClusterName == "" || ng.Name == "" {
		return ""
	}
	return fmt.Sprintf("aws eks describe-nodegroup --cluster-name %s --nodegroup-name %s", shellQuote(ng.ClusterName), shellQuote(ng.Name))
}

func int32PtrString(v *int32) string {
	if v == nil {
		return "unknown"
	}
	return fmt.Sprintf("%d", *v)
}

func emptyDash(v string) string {
	if v == "" {
		return "-"
	}
	return v
}
