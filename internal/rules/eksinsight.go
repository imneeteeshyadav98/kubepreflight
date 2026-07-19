package rules

import (
	"fmt"
	"strings"

	awscol "github.com/imneeteeshyadav98/kubepreflight/internal/collectors/aws"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

const eksInsightFreshnessNote = "AWS-native upgrade readiness checks from Amazon EKS. Insights may be up to 24 hours old; re-check after remediation."

// EKSINSIGHT001 reports AWS-native EKS Upgrade Insights in ERROR status as
// warnings. This is intentionally conservative: provider signal first,
// blocker policy later after real-world validation.
type EKSINSIGHT001 struct{}

func (EKSINSIGHT001) ID() string { return "EKS-INSIGHT-001" }

func (EKSINSIGHT001) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	return eksInsightFindings(sc, targetVersion, "ERROR", findings.SeverityWarning, "EKS-INSIGHT-001")
}

// EKSINSIGHT002 reports AWS-native EKS Upgrade Insights in WARNING status.
type EKSINSIGHT002 struct{}

func (EKSINSIGHT002) ID() string { return "EKS-INSIGHT-002" }

func (EKSINSIGHT002) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	return eksInsightFindings(sc, targetVersion, "WARNING", findings.SeverityWarning, "EKS-INSIGHT-002")
}

// EKSINSIGHT003 surfaces UNKNOWN insight status as informational context.
type EKSINSIGHT003 struct{}

func (EKSINSIGHT003) ID() string { return "EKS-INSIGHT-003" }

func (EKSINSIGHT003) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	return eksInsightFindings(sc, targetVersion, "UNKNOWN", findings.SeverityInfo, "EKS-INSIGHT-003")
}

func eksInsightFindings(sc *ScanContext, targetVersion, status string, severity findings.Severity, ruleID string) ([]findings.Finding, error) {
	if sc.AWS == nil {
		return nil, nil
	}
	var out []findings.Finding
	for _, ins := range sc.AWS.Insights {
		if ins.Status != status {
			continue
		}
		out = append(out, eksInsightFinding(ruleID, severity, ins, targetVersion))
	}
	return out, nil
}

func eksInsightFinding(ruleID string, severity findings.Severity, ins awscol.InsightRecord, targetVersion string) findings.Finding {
	version := firstNonEmpty(ins.KubernetesVersion, targetVersion, "unknown")
	msg := fmt.Sprintf("EKS upgrade insight %q reports %s for Kubernetes %s. Review AWS recommendation before starting the EKS control-plane upgrade.", ins.Name, ins.Status, version)
	if severity == findings.SeverityInfo {
		msg = fmt.Sprintf("EKS upgrade insight %q reports UNKNOWN for Kubernetes %s. Treat this as AWS-native context and verify with a fresh scan before upgrade.", ins.Name, version)
	}

	evidence := []string{
		fmt.Sprintf("insight id: %s", ins.ID),
		fmt.Sprintf("status: %s", ins.Status),
		fmt.Sprintf("kubernetes version: %s", version),
	}
	if ins.Reason != "" {
		evidence = append(evidence, "reason: "+ins.Reason)
	}
	if !ins.LastRefreshTime.IsZero() {
		evidence = append(evidence, "last refreshed: "+ins.LastRefreshTime.UTC().Format("2006-01-02T15:04:05Z"))
	}
	if !ins.LastTransitionTime.IsZero() {
		evidence = append(evidence, "last transition: "+ins.LastTransitionTime.UTC().Format("2006-01-02T15:04:05Z"))
	}
	if ins.Recommendation != "" {
		evidence = append(evidence, "recommendation: "+ins.Recommendation)
	}
	evidence = append(evidence, prefixedDetails("deprecation detail", ins.DeprecationDetails)...)
	evidence = append(evidence, prefixedDetails("add-on compatibility detail", ins.AddonCompatibility)...)
	evidence = append(evidence, "freshness note: "+eksInsightFreshnessNote)

	ref := findings.AWSInsightResource(ins.Category, ins.KubernetesVersion, ins.ID, ins.Name)
	return findings.Finding{
		RuleID:      ruleID,
		Severity:    severity,
		Confidence:  findings.TierProviderReported,
		Message:     msg,
		Resources:   []findings.ResourceReference{ref},
		Evidence:    evidence,
		Remediation: firstNonEmpty(ins.Recommendation, "Review this AWS-native EKS Upgrade Insight in Amazon EKS and confirm with KubePreflight's local findings before upgrading.") + "\n\n" + eksInsightFreshnessNote,
		RemediationDetail: &findings.RemediationDetail{
			SafeFix:       &findings.RemediationAction{Label: "Inspect EKS Upgrade Insight", Steps: []string{"Review the AWS-native EKS Upgrade Insight and any recommendation/details returned by Amazon EKS."}, Command: describeInsightCommand(ins)},
			VerifyCommand: describeInsightCommand(ins),
		},
		Fingerprint: findings.FingerprintV2(ruleID, targetVersion, "", ref),
	}
}

// firstNonEmpty returns the first non-empty value — previously lived in
// api002.go and moved here when the unregistered API-002 rule was deleted,
// since the EKS-INSIGHT rules above are its only remaining callers.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return "(no further detail provided by AWS)"
}

func prefixedDetails(prefix string, details []string) []string {
	out := make([]string, 0, len(details))
	for _, detail := range details {
		if strings.TrimSpace(detail) == "" {
			continue
		}
		out = append(out, prefix+": "+detail)
	}
	return out
}

func describeInsightCommand(ins awscol.InsightRecord) string {
	if ins.ClusterName == "" || ins.ID == "" {
		return ""
	}
	return fmt.Sprintf("aws eks describe-insight --cluster-name %s --id %s", shellQuote(ins.ClusterName), shellQuote(ins.ID))
}
