package rules

import (
	"fmt"

	awscol "kubepreflight/internal/collectors/aws"
	"kubepreflight/internal/findings"
)

// minFreeIPHeadroom is the minimum number of free IPv4 addresses a
// control-plane subnet should have before an upgrade: EKS creates new
// control-plane ENIs during the upgrade, and running out of subnet IPs
// can fail the upgrade. AWS documents that an EKS update requires up to five
// available addresses in the configured cluster subnets.
const minFreeIPHeadroom = 5

// NODE002 flags a control-plane subnet with too little free IP headroom for
// the ENIs an EKS control-plane upgrade creates.
type NODE002 struct{}

func (NODE002) ID() string { return "NODE-002" }

func (NODE002) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	if sc.AWS == nil {
		return nil, nil // AWS enrichment wasn't attempted or was gracefully skipped.
	}

	var out []findings.Finding
	for _, subnet := range sc.AWS.Subnets {
		if subnet.AvailableIPAddressCount >= minFreeIPHeadroom {
			continue
		}
		out = append(out, node002Finding(subnet, targetVersion))
	}
	return out, nil
}

func node002Finding(subnet awscol.SubnetRecord, targetVersion string) findings.Finding {
	msg := fmt.Sprintf(
		"Control-plane subnet %s has only %d free IPv4 address(es) — below the %d-address headroom recommended for the ENIs an upgrade creates; the upgrade can fail outright if the subnet runs out of IPs mid-flight",
		subnet.ID, subnet.AvailableIPAddressCount, minFreeIPHeadroom)

	remediation := "Free up IPs in this subnet before the change window (release unused ENIs/load balancers, or add a secondary CIDR), " +
		"or add an additional control-plane subnet with headroom via a VPC config update. " +
		"Re-run `aws ec2 describe-subnets` afterward to confirm headroom before opening the upgrade window."

	ref := findings.AWSResource("Subnet", subnet.ID, subnet.ID)
	return findings.Finding{
		RuleID:     "NODE-002",
		Severity:   findings.SeverityBlocker,
		Confidence: findings.TierStaticCertain,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		Evidence: []string{
			fmt.Sprintf("available IPv4 addresses: %d", subnet.AvailableIPAddressCount),
			fmt.Sprintf("recommended minimum: %d", minFreeIPHeadroom),
		},
		Remediation: remediation,
		RemediationDetail: &findings.RemediationDetail{
			Changes:        []findings.RemediationChange{{Field: "available IPv4 addresses", Current: fmt.Sprintf("%d", subnet.AvailableIPAddressCount), Required: fmt.Sprintf(">= %d", minFreeIPHeadroom)}},
			SafeFix:        &findings.RemediationAction{Label: "Safe fix", Steps: []string{"Free unused addresses or update the EKS control-plane subnet configuration to use subnets with sufficient headroom."}, Command: fmt.Sprintf("aws ec2 describe-subnets --subnet-ids %s --query 'Subnets[*].[SubnetId,AvailableIpAddressCount]' --output table", subnet.ID)},
			VerifyCommand:  fmt.Sprintf("aws ec2 describe-subnets --subnet-ids %s --query 'Subnets[0].AvailableIpAddressCount' --output text", subnet.ID),
			ExpectedResult: fmt.Sprintf("%d or greater", minFreeIPHeadroom),
		},
		Fingerprint: findings.FingerprintV2("NODE-002", targetVersion, "", ref),
	}
}
