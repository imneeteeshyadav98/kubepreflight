// Package findings defines the KubePreflight finding schema: severities,
// confidence tiers, structured resource references, and fingerprints.
package findings

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Severity is the blocking level of a finding.
type Severity string

const (
	SeverityBlocker Severity = "Blocker"
	SeverityWarning Severity = "Warning"
	SeverityInfo    Severity = "Info"
)

// ConfidenceTier labels how a finding was established.
type ConfidenceTier string

const (
	TierStaticCertain    ConfidenceTier = "STATIC_CERTAIN"
	TierProviderReported ConfidenceTier = "PROVIDER_REPORTED"
	TierObserved         ConfidenceTier = "OBSERVED"
	TierInferred         ConfidenceTier = "INFERRED"
)

// Plane identifies where one occurrence of a finding's subject was observed.
type Plane string

const (
	PlaneLive     Plane = "live"
	PlaneManifest Plane = "manifest"
	PlaneAWS      Plane = "aws"
)

// ResourceScope distinguishes a genuinely cluster-scoped Kubernetes object
// from a namespaced manifest whose namespace was omitted and is therefore
// unsafe to correlate.
type ResourceScope string

const (
	ScopeCluster    ResourceScope = "cluster"
	ScopeNamespaced ResourceScope = "namespaced"
)

// ResourceReference is a validated tagged-union-shaped reference. Plane
// selects which fields are meaningful: live Kubernetes, manifest Kubernetes,
// or AWS/provider evidence. Findings keep every occurrence reference even
// when several occurrences correlate to one conceptual issue.
type ResourceReference struct {
	Plane Plane `json:"plane"`

	Kind       string        `json:"kind,omitempty"`
	Scope      ResourceScope `json:"scope,omitempty"`
	Namespace  string        `json:"namespace,omitempty"`
	Name       string        `json:"name,omitempty"`
	UID        string        `json:"uid,omitempty"`
	SourcePath string        `json:"sourcePath,omitempty"`

	Category          string `json:"category,omitempty"`
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`
	ProviderID        string `json:"providerId,omitempty"`
	ProviderName      string `json:"providerName,omitempty"`
}

func LiveResource(kind string, scope ResourceScope, namespace, name, uid string) ResourceReference {
	return ResourceReference{Plane: PlaneLive, Kind: kind, Scope: scope, Namespace: namespace, Name: name, UID: uid}
}

func ManifestResource(kind string, scope ResourceScope, namespace, name, sourcePath string) ResourceReference {
	return ResourceReference{Plane: PlaneManifest, Kind: kind, Scope: scope, Namespace: namespace, Name: name, SourcePath: sourcePath}
}

func AWSResource(kind, name, providerID string) ResourceReference {
	return ResourceReference{Plane: PlaneAWS, Kind: kind, Name: name, ProviderID: providerID}
}

func AWSInsightResource(category, kubernetesVersion, providerID, providerName string) ResourceReference {
	return ResourceReference{
		Plane: PlaneAWS, Kind: "EKSUpgradeInsight", Name: providerName,
		Category: category, KubernetesVersion: kubernetesVersion,
		ProviderID: providerID, ProviderName: providerName,
	}
}

// Validate rejects cross-plane field combinations. Rule code uses the
// constructors above; validation also protects future JSON/schema consumers.
func (r ResourceReference) Validate() error {
	switch r.Plane {
	case PlaneLive:
		if r.Kind == "" || r.Name == "" || r.UID == "" || r.SourcePath != "" || r.ProviderID != "" {
			return fmt.Errorf("invalid live resource reference")
		}
	case PlaneManifest:
		if r.Kind == "" || r.Name == "" || r.SourcePath == "" || r.UID != "" || r.ProviderID != "" {
			return fmt.Errorf("invalid manifest resource reference")
		}
	case PlaneAWS:
		if r.ProviderID == "" || r.UID != "" || r.SourcePath != "" {
			return fmt.Errorf("invalid AWS resource reference")
		}
		if r.Kind == "EKSUpgradeInsight" && (r.Category == "" || r.KubernetesVersion == "") {
			return fmt.Errorf("invalid AWS insight reference")
		}
	default:
		return fmt.Errorf("unknown resource plane %q", r.Plane)
	}
	return nil
}

// OccurrenceKey identifies this exact observation/provenance, not merely the
// conceptual object it may correlate with on another plane.
func (r ResourceReference) OccurrenceKey() string {
	switch r.Plane {
	case PlaneLive:
		return canonicalKey("occurrence", "live", r.UID)
	case PlaneManifest:
		return canonicalKey("occurrence", "manifest", r.SourcePath, r.Kind, r.Namespace, r.Name)
	case PlaneAWS:
		return canonicalKey("occurrence", "aws", r.ProviderID)
	default:
		return canonicalKey("occurrence", string(r.Plane), r.Kind, r.Namespace, r.Name)
	}
}

// ConceptKey identifies an issue subject across evidence planes. A namespaced
// object with no explicit namespace deliberately has no concept key: apply-time
// namespace selection cannot be guessed safely.
func (r ResourceReference) ConceptKey() (string, bool) {
	switch r.Plane {
	case PlaneLive, PlaneManifest:
		if r.Kind == "" || r.Name == "" {
			return "", false
		}
		if r.Scope == ScopeNamespaced && r.Namespace == "" {
			return "", false
		}
		if r.Scope != ScopeCluster && r.Scope != ScopeNamespaced {
			return "", false
		}
		return canonicalKey("k8s-object", r.Kind, r.Namespace, r.Name), true
	case PlaneAWS:
		if r.ProviderID == "" {
			return "", false
		}
		if r.Kind == "EKSUpgradeInsight" {
			if r.Category == "" || r.KubernetesVersion == "" {
				return "", false
			}
			return canonicalKey("aws-insight", r.Category, r.KubernetesVersion, r.ProviderID), true
		}
		return canonicalKey("aws-resource", r.Kind, r.ProviderID), true
	default:
		return "", false
	}
}

// RemediationChange is one field-level current-vs-required pair, e.g. an
// apiVersion bump or a disruptionsAllowed target.
type RemediationChange struct {
	Field    string `json:"field,omitempty"`
	Current  string `json:"current,omitempty"`
	Required string `json:"required,omitempty"`
}

// RemediationAction is one concrete course of action: a safe fix or a
// clearly-marked emergency workaround. Steps is prose (a ladder of options,
// a caveat); Command is the exact copy-pastable command line(s), when one
// exists.
type RemediationAction struct {
	Label   string   `json:"label"`
	Steps   []string `json:"steps,omitempty"`
	Command string   `json:"command,omitempty"`
	Risky   bool     `json:"risky,omitempty"`
}

// RemediationDetail is the structured counterpart to Finding.Remediation,
// populated only by rules that can honestly derive current/required values,
// diffs, and commands from data already in hand — never guessed. A nil
// RemediationDetail means the finding still has plain-text Remediation;
// every renderer must keep working when this is nil.
type RemediationDetail struct {
	AffectedFile string              `json:"affectedFile,omitempty"`
	Changes      []RemediationChange `json:"changes,omitempty"`
	Diff         string              `json:"diff,omitempty"`
	SafeFix      *RemediationAction  `json:"safeFix,omitempty"`
	Emergency    *RemediationAction  `json:"emergency,omitempty"`
	// BreakGlass is the last-resort, cluster-is-bricked option (e.g.
	// deleting a webhook configuration entirely) — always Risky, and
	// always more severe than Emergency's temporary mitigation.
	BreakGlass     *RemediationAction `json:"breakGlass,omitempty"`
	VerifyCommand  string             `json:"verifyCommand,omitempty"`
	ExpectedResult string             `json:"expectedResult,omitempty"`
}

// Finding is a single evidence-backed risk output by a rule. Resources is a
// list because one conceptual finding may have several occurrences (API-001)
// or inherently involve several resources (PDB-002).
type Finding struct {
	RuleID            string              `json:"ruleId"`
	Severity          Severity            `json:"severity"`
	Confidence        ConfidenceTier      `json:"confidence"`
	Message           string              `json:"message"`
	Resources         []ResourceReference `json:"resources"`
	Evidence          []string            `json:"evidence,omitempty"`
	Remediation       string              `json:"remediation,omitempty"`
	RemediationDetail *RemediationDetail  `json:"remediationDetail,omitempty"`
	// GlobalBlocker marks a finding whose condition can block other
	// remediation commands (kubectl apply/patch/scale, Helm upgrades)
	// from succeeding at all — e.g. a fail-closed webhook with no
	// healthy backend. omitempty keeps every other rule's JSON output
	// byte-identical.
	GlobalBlocker bool   `json:"globalBlocker,omitempty"`
	Fingerprint   string `json:"fingerprint"`
}

func (f Finding) Validate() error {
	if f.RuleID == "" {
		return fmt.Errorf("finding has no rule ID")
	}
	if f.Severity != SeverityBlocker && f.Severity != SeverityWarning && f.Severity != SeverityInfo {
		return fmt.Errorf("finding %s has invalid severity %q", f.RuleID, f.Severity)
	}
	if f.Confidence != TierStaticCertain && f.Confidence != TierProviderReported && f.Confidence != TierObserved && f.Confidence != TierInferred {
		return fmt.Errorf("finding %s has invalid confidence %q", f.RuleID, f.Confidence)
	}
	if len(f.Resources) == 0 {
		return fmt.Errorf("finding %s has no resource references", f.RuleID)
	}
	for i, ref := range f.Resources {
		if err := ref.Validate(); err != nil {
			return fmt.Errorf("finding %s resource %d: %w", f.RuleID, i, err)
		}
	}
	if f.Fingerprint == "" {
		return fmt.Errorf("finding %s has no fingerprint", f.RuleID)
	}
	if f.RemediationDetail != nil && f.RemediationDetail.BreakGlass != nil && !f.RemediationDetail.BreakGlass.Risky {
		return fmt.Errorf("finding %s has a break-glass action not marked risky", f.RuleID)
	}
	return nil
}

// Fingerprint is the legacy pre-structured-identity fingerprint. It remains
// available only to make the domain separation testable; new findings use
// FingerprintV2.
func Fingerprint(ruleID, resourceUID, targetVersion string) string {
	h := sha256.New()
	h.Write([]byte(strings.Join([]string{ruleID, resourceUID, targetVersion}, "|")))
	return hex.EncodeToString(h.Sum(nil))
}

// FingerprintV2 hashes the rule, target, optional within-resource issue
// discriminator (for example a webhook block name), and sorted conceptual
// resource keys. Occurrence keys are used only where conservative matching
// intentionally yields no concept key, such as an omitted manifest namespace.
func FingerprintV2(ruleID, targetVersion, discriminator string, refs ...ResourceReference) string {
	keys := make([]string, 0, len(refs))
	seen := map[string]bool{}
	for _, ref := range refs {
		k, ok := ref.ConceptKey()
		if !ok {
			k = ref.OccurrenceKey()
		}
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	parts := []string{"finding-v2", ruleID, targetVersion, discriminator}
	parts = append(parts, keys...)
	return canonicalKey(parts...)
}

func canonicalKey(parts ...string) string {
	raw, _ := json.Marshal(parts) // []string cannot fail to marshal.
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
