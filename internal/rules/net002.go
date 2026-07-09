package rules

import (
	"fmt"

	awscol "kubepreflight/internal/collectors/aws"
	"kubepreflight/internal/findings"
)

// NET002 flags a cluster whose referenced security group or VPC no longer
// exists. These are hard EKS control-plane upgrade-failure preconditions
// per AWS's own troubleshooting documentation (SecurityGroupNotFound,
// VpcIdNotFound) — not soft warnings — and a natural NODE-002 sibling: both
// verify infrastructure preconditions the control-plane upgrade depends
// on, using the same AWS collector. It was added after real-world research
// surfaced missing network resources as a common EKS upgrade failure mode
// alongside IP exhaustion (NODE-002).
type NET002 struct{}

func (NET002) ID() string { return "NET-002" }

func (NET002) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	if sc.AWS == nil {
		return nil, nil // AWS enrichment wasn't attempted or was gracefully skipped.
	}

	var out []findings.Finding
	for _, issue := range sc.AWS.NetworkPreflightIssues {
		out = append(out, net002Finding(issue, targetVersion))
	}
	return out, nil
}

func net002Finding(issue awscol.NetworkPreflightIssue, targetVersion string) findings.Finding {
	awsErrorCode := "InvalidVpcID.NotFound"
	describeCmd := fmt.Sprintf("aws ec2 describe-vpcs --vpc-ids %s", shellQuote(issue.ID))
	if issue.Kind == "SecurityGroup" {
		awsErrorCode = "InvalidGroup.NotFound"
		describeCmd = fmt.Sprintf("aws ec2 describe-security-groups --group-ids %s", shellQuote(issue.ID))
	}

	msg := fmt.Sprintf(
		"Cluster references %s %s, which no longer exists (%s) — this is a hard EKS control-plane upgrade-failure precondition per AWS's own troubleshooting documentation, not a soft warning",
		issue.Kind, issue.ID, awsErrorCode)

	remediation := fmt.Sprintf(
		"Restore or recreate the missing %s (%s) before the change window, or update the cluster's VPC config if it was intentionally replaced — "+
			"an EKS cluster cannot be reassigned to a different VPC after creation. Verify with `%s`.",
		issue.Kind, issue.ID, describeCmd)

	ref := findings.AWSResource(issue.Kind, issue.ID, issue.ID)
	return findings.Finding{
		RuleID:     "NET-002",
		Severity:   findings.SeverityBlocker,
		Confidence: findings.TierStaticCertain,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		Evidence: []string{
			fmt.Sprintf("resource kind: %s", issue.Kind),
			fmt.Sprintf("resource id: %s", issue.ID),
			fmt.Sprintf("AWS error code: %s", awsErrorCode),
		},
		Remediation: remediation,
		RemediationDetail: &findings.RemediationDetail{
			Changes:        []findings.RemediationChange{{Field: issue.Kind + " existence", Current: "not found", Required: "available to the EKS cluster"}},
			SafeFix:        &findings.RemediationAction{Label: "Safe fix", Steps: []string{"Restore the referenced resource or update the supported EKS VPC configuration before upgrading; an EKS cluster cannot be moved to another VPC."}, Command: describeCmd},
			VerifyCommand:  describeCmd,
			ExpectedResult: "resource is returned without a NotFound error",
		},
		Fingerprint: findings.FingerprintV2("NET-002", targetVersion, "", ref),
	}
}
