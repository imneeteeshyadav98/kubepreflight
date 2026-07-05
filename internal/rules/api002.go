package rules

import (
	"fmt"
	"strings"

	awscol "kubepreflight/internal/collectors/aws"
	"kubepreflight/internal/findings"
)

// insightStalenessCaveat must appear in every API-002 finding's evidence.
// EKS Upgrade Insights are computed from a rolling 30-day audit-log
// lookback, refreshed on a ~24-hour cadence (deep dive Section 3.2) — this
// is the single most important caveat for trusting this rule's output, and
// it's a documented must-have, not optional context.
const insightStalenessCaveat = "EKS Upgrade Insights are computed from a rolling 30-day audit-log lookback, refreshed on a ~24-hour cadence. " +
	"If the underlying issue has already been fixed, this insight can remain WARNING/ERROR for up to 30 days until the old audit-log entries age out. " +
	"A clean local scan (e.g. API-001) alongside a lingering AWS-reported issue here is a sign the fix already landed, not a contradiction."

// API002 ingests EKS Upgrade Insights (UPGRADE_READINESS category)
// relevant to the scan's target version. Confidence is always
// PROVIDER_REPORTED: this rule relays AWS's own signal rather than
// computing anything itself (deep dive Section 3, check API-002).
type API002 struct{}

func (API002) ID() string { return "API-002" }

func (API002) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	if sc.AWS == nil {
		return nil, nil // AWS enrichment wasn't attempted or was gracefully skipped.
	}

	var out []findings.Finding
	for _, ins := range sc.AWS.Insights {
		out = append(out, api002Finding(ins, targetVersion))
	}
	return out, nil
}

func api002Finding(ins awscol.InsightRecord, targetVersion string) findings.Finding {
	severity := findings.SeverityWarning
	if ins.Status == "ERROR" {
		severity = findings.SeverityBlocker
	}

	msg := fmt.Sprintf("EKS Upgrade Insight %q is %s for target version %s: %s",
		ins.Name, ins.Status, targetVersion, firstNonEmpty(ins.Recommendation, ins.Reason, ins.Description))

	evidence := []string{
		fmt.Sprintf("insight id: %s", ins.ID),
		fmt.Sprintf("category: %s", ins.Category),
		fmt.Sprintf("status: %s", ins.Status),
	}
	if ins.Reason != "" {
		evidence = append(evidence, fmt.Sprintf("reason: %s", ins.Reason))
	}
	if !ins.LastRefreshTime.IsZero() {
		evidence = append(evidence, fmt.Sprintf("last refreshed: %s", ins.LastRefreshTime.Format("2006-01-02T15:04:05Z")))
	}
	evidence = append(evidence, "staleness caveat: "+insightStalenessCaveat)

	// AWS's own recommendation is the primary instruction; the staleness
	// caveat is context, not a continuation of it, so it's kept clearly
	// separate rather than run on into the same sentence.
	remediation := insightStalenessCaveat
	if ins.Recommendation != "" {
		action := strings.TrimSpace(ins.Recommendation)
		if !strings.HasSuffix(action, ".") {
			action += "."
		}
		remediation = action + "\n\n" + insightStalenessCaveat
	}

	ref := findings.AWSInsightResource(ins.Category, ins.KubernetesVersion, ins.ID, ins.Name)
	var detail *findings.RemediationDetail
	if ins.ClusterName != "" {
		detail = &findings.RemediationDetail{
			SafeFix:        &findings.RemediationAction{Label: "Provider-reported recommendation", Steps: []string{firstNonEmpty(ins.Recommendation, ins.Reason, ins.Description), "Confirm locally because EKS Upgrade Insights can remain stale for up to 30 days."}},
			VerifyCommand:  fmt.Sprintf("aws eks describe-insight --cluster-name %s --id %s", shellQuote(ins.ClusterName), shellQuote(ins.ID)),
			ExpectedResult: "status is PASSING after the provider refresh window, corroborated by a fresh local scan",
		}
	}
	return findings.Finding{
		RuleID:            "API-002",
		Severity:          severity,
		Confidence:        findings.TierProviderReported,
		Message:           msg,
		Resources:         []findings.ResourceReference{ref},
		Evidence:          evidence,
		Remediation:       remediation,
		RemediationDetail: detail,
		Fingerprint:       findings.FingerprintV2("API-002", targetVersion, "", ref),
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return "(no further detail provided by AWS)"
}
