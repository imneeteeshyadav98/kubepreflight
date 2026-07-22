package rules

import (
	"fmt"

	awscol "github.com/imneeteeshyadav98/kubepreflight/internal/collectors/aws"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

// NET002 flags a cluster whose referenced security group or VPC no longer
// exists. These are EKS control-plane upgrade-failure preconditions per AWS's
// own troubleshooting documentation (SecurityGroupNotFound, VpcIdNotFound), and
// a natural NODE-002 sibling: both verify infrastructure preconditions the
// control-plane upgrade depends on, using the same AWS collector. Their upgrade
// gate is context-aware: they block control-plane/full-platform operations and
// stay review evidence for unrelated or unspecified operations.
type NET002 struct{}

func (NET002) ID() string { return "NET-002" }

func (NET002) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	if sc.AWS == nil {
		return nil, nil // AWS enrichment wasn't attempted or was gracefully skipped.
	}

	var out []findings.Finding
	for _, issue := range sc.AWS.NetworkPreflightIssues {
		out = append(out, net002Finding(issue, targetVersion, scanUpgradeContext(sc)))
	}
	return out, nil
}

func net002Finding(issue awscol.NetworkPreflightIssue, targetVersion string, upgradeContext findings.UpgradeContext) findings.Finding {
	severity, gate := eksControlPlanePreconditionGate(upgradeContext)
	awsErrorCode := "InvalidVpcID.NotFound"
	describeCmd := fmt.Sprintf("aws ec2 describe-vpcs --vpc-ids %s", shellQuote(issue.ID))
	if issue.Kind == "SecurityGroup" {
		awsErrorCode = "InvalidGroup.NotFound"
		describeCmd = fmt.Sprintf("aws ec2 describe-security-groups --group-ids %s", shellQuote(issue.ID))
	}

	msg := fmt.Sprintf(
		"Cluster references %s %s, which no longer exists (%s) — this can fail an EKS control-plane or full-platform upgrade when that operation depends on the missing network resource",
		issue.Kind, issue.ID, awsErrorCode)

	remediation := fmt.Sprintf(
		"Restore or recreate the missing %s (%s) before an EKS control-plane or full-platform upgrade, or update the cluster's VPC config if it was intentionally replaced — "+
			"an EKS cluster cannot be reassigned to a different VPC after creation. For audit-only, worker-rollout, workload-restart, or unspecified contexts, treat this as provider precondition evidence to review rather than a confirmed blocker for the selected operation. Verify with `%s`.",
		issue.Kind, issue.ID, describeCmd)

	ref := findings.AWSResource(issue.Kind, issue.ID, issue.ID)
	return findings.Finding{
		RuleID:     "NET-002",
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
