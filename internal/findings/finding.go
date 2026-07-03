// Package findings defines the KubePreflight finding schema: severities,
// confidence tiers, and the fingerprint used for dedup/waivers across scans.
package findings

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Severity is the blocking level of a finding.
type Severity string

const (
	SeverityBlocker Severity = "Blocker"
	SeverityWarning Severity = "Warning"
	SeverityInfo    Severity = "Info"
)

// ConfidenceTier labels how a finding was established. Only the two tiers
// available from static/manifest and provider-relayed evidence ship in v0.1;
// OBSERVED and INFERRED are added once the probe and telemetry collectors
// land (v0.2+).
type ConfidenceTier string

const (
	// TierStaticCertain is provable directly from manifests or live objects.
	TierStaticCertain ConfidenceTier = "STATIC_CERTAIN"
	// TierProviderReported is relayed from an AWS API (e.g. EKS Insights),
	// with any staleness/caveats carried in the finding's evidence.
	TierProviderReported ConfidenceTier = "PROVIDER_REPORTED"
)

// Resource identifies the Kubernetes object a finding is about.
type Resource struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
	UID       string `json:"uid"`
}

// Finding is a single piece of evidence-backed risk output by a rule.
type Finding struct {
	RuleID      string         `json:"ruleId"`
	Severity    Severity       `json:"severity"`
	Confidence  ConfidenceTier `json:"confidence"`
	Message     string         `json:"message"`
	Resource    Resource       `json:"resource"`
	Evidence    []string       `json:"evidence,omitempty"`
	Remediation string         `json:"remediation,omitempty"`
	Fingerprint string         `json:"fingerprint"`
}

// Fingerprint derives the dedup/waiver key for a finding: rule ID + resource
// UID + target version, per the product spec (Section 15 / 18.2 of the deep
// dive). It is deterministic across scans of the same cluster/target.
func Fingerprint(ruleID, resourceUID, targetVersion string) string {
	h := sha256.New()
	h.Write([]byte(strings.Join([]string{ruleID, resourceUID, targetVersion}, "|")))
	return hex.EncodeToString(h.Sum(nil))
}
