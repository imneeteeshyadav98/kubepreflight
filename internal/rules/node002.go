package rules

import (
	"fmt"

	awscol "github.com/imneeteeshyadav98/kubepreflight/internal/collectors/aws"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
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
		out = append(out, node002Finding(subnet, targetVersion, scanUpgradeContext(sc)))
	}
	return out, nil
}

func node002Finding(subnet awscol.SubnetRecord, targetVersion string, upgradeContext findings.UpgradeContext) findings.Finding {
	severity, gate := eksControlPlanePreconditionGate(upgradeContext)
	msg := fmt.Sprintf(
		"Control-plane subnet %s has only %d free IPv4 address(es) — below the %d-address headroom recommended for the ENIs an EKS control-plane or full-platform upgrade creates; that operation can fail if the subnet runs out of IPs mid-flight",
		subnet.ID, subnet.AvailableIPAddressCount, minFreeIPHeadroom)

	remediation := "Free up IPs in this subnet before an EKS control-plane or full-platform upgrade (release unused ENIs/load balancers, or add a secondary CIDR), " +
		"or add an additional control-plane subnet with headroom via a VPC config update. " +
		"Re-run `aws ec2 describe-subnets` afterward to confirm headroom. For audit-only, worker-rollout, workload-restart, or unspecified contexts, treat this as provider precondition evidence to review rather than a confirmed blocker for the selected operation."

	ref := findings.AWSResource("Subnet", subnet.ID, subnet.ID)
	return findings.Finding{
		RuleID:     "NODE-002",
		Severity:   severity,
		Confidence: findings.TierStaticCertain,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		ImpactScopes: []findings.ImpactScope{
			findings.ImpactScopeControlPlaneUpgrade,
			findings.ImpactScopeFutureMaintenance,
		},
		UpgradeGate: gate,
		Evidence: []string{
			fmt.Sprintf("available IPv4 addresses: %d", subnet.AvailableIPAddressCount),
			fmt.Sprintf("recommended minimum: %d", minFreeIPHeadroom),
		},
		Remediation: remediation,
		RemediationDetail: &findings.RemediationDetail{
			Changes:        []findings.RemediationChange{{Field: "available IPv4 addresses", Current: fmt.Sprintf("%d", subnet.AvailableIPAddressCount), Required: fmt.Sprintf(">= %d", minFreeIPHeadroom)}},
			SafeFix:        &findings.RemediationAction{Label: "Safe fix", Steps: []string{"Free unused addresses or update the EKS control-plane subnet configuration to use subnets with sufficient headroom."}, Command: fmt.Sprintf("aws ec2 describe-subnets --subnet-ids %s --query 'Subnets[*].[SubnetId,AvailableIpAddressCount]' --output table", shellQuote(subnet.ID))},
			VerifyCommand:  fmt.Sprintf("aws ec2 describe-subnets --subnet-ids %s --query 'Subnets[0].AvailableIpAddressCount' --output text", shellQuote(subnet.ID)),
			ExpectedResult: fmt.Sprintf("%d or greater", minFreeIPHeadroom),
		},
		Fingerprint: findings.FingerprintV2("NODE-002", targetVersion, "", ref),
	}
}
