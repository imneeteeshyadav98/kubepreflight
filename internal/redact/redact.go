// Package redact removes AWS account IDs/ARNs and EC2-style internal node
// hostnames from an already-fully-built report or assessment, for users who
// intend to share generated evidence (findings.json, report.html,
// rollback-assessment.json, upgrade-plan.json) outside their organization.
//
// This is presentation-layer only, by design: every function here operates
// on a *findings.Report/*rollback.Assessment/*plan.PlanReport after every
// rule has already evaluated the real values and every fingerprint/
// comparison key has already been derived from them (fingerprints are
// computed during rule evaluation, long before a CLI command builds its
// final report — see findings.FingerprintV2). Redacting afterward can never
// change which findings match across two scans in `kubepreflight compare`,
// never changes RuleID/Severity/Confidence/Priority/scores/verdicts/exit
// codes, and is never applied during collection or rule evaluation.
//
// It exists because the default output does not redact anything — real
// AWS account IDs, cluster ARNs, and internal node hostnames appear
// verbatim in generated evidence. Opt in per-command with
// --redact-sensitive-identifiers.
package redact

import "regexp"

// arnPattern matches AWS ARNs generally (not just EKS), since the account
// ID they carry is the actual thing worth removing — an ARN for any AWS
// service embeds the same 12-digit account ID.
var arnPattern = regexp.MustCompile(`arn:aws:[a-zA-Z0-9][a-zA-Z0-9.-]*:[a-z0-9-]*:\d{12}:[a-zA-Z0-9/:_.\-]+`)

// hostnamePattern matches the EC2-assigned node hostname format Kubernetes
// uses as the node name by default on AWS: either the plain
// ip-10-0-1-100.ec2.internal form, or the region-qualified
// ip-10-0-1-100.us-east-1.compute.internal form some VPC DNS
// configurations use instead — these are two distinct suffix shapes, not
// one pattern with an optional middle segment.
var hostnamePattern = regexp.MustCompile(`\bip-\d{1,3}-\d{1,3}-\d{1,3}-\d{1,3}\.(?:ec2\.internal|[a-zA-Z0-9-]+\.compute\.internal)\b`)

// accountIDPattern matches a bare 12-digit AWS account ID that appears
// outside of an ARN — e.g. free text like "AccessDenied for account
// 164761934067" — which arnPattern alone cannot catch since it requires
// the "arn:aws:" prefix. Applied after arnPattern (see Text), so the
// account ID embedded in an actual ARN is already gone by the time this
// runs and is never double-matched. \b on both sides means a digit run
// adjacent to a letter (as in a hex fingerprint/UID) never matches, since
// letters count as word characters too and there is no boundary there.
var accountIDPattern = regexp.MustCompile(`\b\d{12}\b`)

const (
	ARNPlaceholder       = "[redacted-arn]"
	HostnamePlaceholder  = "[redacted-node-hostname]"
	AccountIDPlaceholder = "[redacted-account-id]"
)

// Text redacts every AWS ARN, standalone 12-digit AWS account ID, and
// EC2-style internal node hostname found in s. A string with none of
// these patterns present is returned unchanged (and is not reallocated),
// so it's always safe to call on every string field — ordinary resource
// names like "critical-app-pdb" never match any pattern and pass through
// as-is.
func Text(s string) string {
	if s == "" {
		return s
	}
	s = arnPattern.ReplaceAllString(s, ARNPlaceholder)
	s = hostnamePattern.ReplaceAllString(s, HostnamePlaceholder)
	s = accountIDPattern.ReplaceAllString(s, AccountIDPlaceholder)
	return s
}

// Strings redacts every element of ss, preserving nil vs. empty-slice
// distinction (relevant for `omitempty` JSON fields).
func Strings(ss []string) []string {
	if ss == nil {
		return nil
	}
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = Text(s)
	}
	return out
}

// StringMapValues redacts every value (never the keys — keys here are
// AWS-defined field names like "kubernetesVersion", never identifiers) of
// m in place.
func StringMapValues(m map[string]string) {
	for k, v := range m {
		m[k] = Text(v)
	}
}
