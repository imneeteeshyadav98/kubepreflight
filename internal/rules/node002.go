package rules

import (
	"fmt"

	awscol "kubepreflight/internal/collectors/aws"
	"kubepreflight/internal/findings"
)

// minFreeIPHeadroom is the minimum number of free IPv4 addresses a
// control-plane subnet should have before an upgrade: EKS creates new
// control-plane ENIs during the upgrade, and running out of subnet IPs
// fails the upgrade outright (deep dive Section 2.2, check NODE-002). This
// is a conservative starting heuristic, not an AWS-published hard minimum —
// expect to tune it once pilot data exists (Week 8).
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
		Fingerprint: findings.FingerprintV2("NODE-002", targetVersion, "", ref),
	}
}
