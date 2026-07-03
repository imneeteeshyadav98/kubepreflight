# Structured resource identity (item #2 design)

Status: approved and implemented in item #2.

## Decision to make

A single key cannot safely mean both "this exact observation" and "the same
conceptual issue". A live UID identifies one object incarnation, a manifest path
identifies source provenance, and an AWS insight ID identifies a provider
record. None of those can correlate all planes. The model therefore needs two
layers:

- an **occurrence key**, which preserves where a reference came from; and
- a **concept key**, which is deliberately less specific and may correlate
  occurrences across planes.

A finding fingerprint is then derived from the rule, target version, and the
sorted set of concept keys involved in the issue. The finding retains every
occurrence reference, so deduplication does not discard evidence or source
paths.

In the examples below, `key(...)` means SHA-256 of a canonical JSON array of
the supplied strings. Using an encoded tuple avoids delimiter and empty-value
ambiguities.

## Option A: one tagged struct with plane-specific optional fields

```go
type Plane string

const (
    PlaneLive     Plane = "live"
    PlaneManifest Plane = "manifest"
    PlaneAWS      Plane = "aws"
)

type ResourceReference struct {
    Plane Plane `json:"plane"`

    // Kubernetes live/manifest fields.
    Kind       string `json:"kind,omitempty"`
    Namespace  string `json:"namespace,omitempty"`
    Name       string `json:"name,omitempty"`
    UID        string `json:"uid,omitempty"`        // live only
    SourcePath string `json:"sourcePath,omitempty"` // manifest only

    // AWS Insight fields.
    Category          string `json:"category,omitempty"`
    KubernetesVersion string `json:"kubernetesVersion,omitempty"`
    ProviderID        string `json:"providerId,omitempty"`
    ProviderName      string `json:"providerName,omitempty"`
}
```

Validation must enforce exactly one valid shape:

- live: `Kind`, `Name`, and `UID`; no source/provider fields;
- manifest: `Kind`, `Name`, and `SourcePath`; no UID/provider fields; and
- AWS: `Category`, `KubernetesVersion`, and `ProviderID`; no Kubernetes object
  fields.

Key derivation:

```text
live occurrence     = key("occurrence", "live", UID)
manifest occurrence = key("occurrence", "manifest", SourcePath,
                          Kind, Namespace, Name)
AWS occurrence      = key("occurrence", "aws", ProviderID)

live concept        = key("k8s-object", Kind, Namespace, Name)
manifest concept    = key("k8s-object", Kind, Namespace, Name)
AWS concept         = key("aws-insight", Category, KubernetesVersion,
                          ProviderID)

finding fingerprint = key("finding-v2", RuleID, TargetVersion,
                          optional issue discriminator,
                          sorted(unique(concept keys))...)
```

For API-001, live and manifest occurrences with equal
`Kind+Namespace+Name` consequently produce the same fingerprint. Multiple
manifest paths also merge into that finding, while all paths remain in its
reference list.

The optional discriminator distinguishes repeated sub-resource issues under one
resource (for example, two webhook blocks in one webhook configuration). It is
empty for API-001 and PDB-002; it cannot alter cross-plane resource matching.

`Category+KubernetesVersion` is useful AWS scope, but is not an issue identity:
the current collector can return several insights within the same category and
version. Omitting `ProviderID` would merge unrelated API-002 findings. If AWS
ever omits an ID, the fallback must include a normalized provider name; it must
not silently fall back to the pair alone.

Advantages: simple JSON, easy renderer/waiver consumption, and incremental
migration from the current concrete `Resource`. Disadvantage: Go permits
invalid field combinations until runtime validation.

## Option B: a small interface with one implementation per shape

```go
type ResourceReference interface {
    Plane() Plane
    OccurrenceKey() string
    ConceptKey() (string, bool)
}

type LiveKubernetesReference struct {
    Kind, Namespace, Name, UID string
}

type ManifestKubernetesReference struct {
    Kind, Namespace, Name, SourcePath string
}

type AWSInsightReference struct {
    Category, KubernetesVersion, ProviderID, ProviderName string
}
```

The methods derive the same keys as Option A:

```text
LiveKubernetesReference:
  occurrence = key("occurrence", "live", UID)
  concept    = key("k8s-object", Kind, Namespace, Name)

ManifestKubernetesReference:
  occurrence = key("occurrence", "manifest", SourcePath,
                   Kind, Namespace, Name)
  concept    = key("k8s-object", Kind, Namespace, Name)

AWSInsightReference:
  occurrence = key("occurrence", "aws", ProviderID)
  concept    = key("aws-insight", Category, KubernetesVersion, ProviderID)
```

Advantages: invalid cross-plane combinations are difficult to construct, and
each type owns its identity rules. Disadvantages: interface slices need custom
tagged JSON marshal/unmarshal, exhaustive rendering is less obvious, and the
public findings schema becomes harder for SARIF, waiver-file, and ticketing
consumers to evolve.

## Recommendation

Use **Option A**, but treat it as a validated tagged union: constructors per
plane, unexported mutation where practical, and validation before a finding is
accepted. Its stable, explicit JSON is more valuable here than interface purity
because reports, waivers, SARIF, and ticket dedup all consume this data. Store
references as a list on a finding; do not overload one synthetic resource to
represent a relationship.

The implementation should version the new fingerprint domain (`finding-v2`),
because changing from UID/source keys to concept keys intentionally changes
existing fingerprints.

## Cross-plane matching rule for API-001

The initial rule should be deliberately literal:

1. Match only exact `Kind + Namespace + Name` after whitespace/case-preserving
   Kubernetes metadata normalization. Do not use fuzzy names, source paths,
   labels, or templating guesses.
2. Different explicit namespaces never match, even if the names and kinds do.
   Thus a rendered manifest for `staging/foo` does not match live `prod/foo`.
3. A manifest namespace that is unresolved, templated, or omitted for a
   namespaced kind has no safe concept key and must stay separate. Do not assume
   `default`: apply-time namespace/context may choose something else.
4. Empty namespace is matchable only when the kind is known to be
   cluster-scoped. That requires scope knowledge from the catalog/discovery;
   empty must not ambiguously mean both cluster-scoped and unknown.
5. A same-identity manifest and live object match even though the manifest may
   never have been applied. The merged finding must retain both references and
   describe them as two observations, not claim that one created the other.

Rule 5 carries an assumption that the supplied manifest inputs represent the
cluster being scanned. If users scan several environment overlays at once,
unrelated desired objects with the same identity can merge. The safe initial
contract is to document that assumption; an alternative is to require an
explicit manifest-to-cluster scope before enabling cross-plane matching.

Kind-only type identity also assumes the locked built-in API catalog has no
same-Kind collision across unrelated API groups. Before this mechanism is
extended to arbitrary CRDs, the concept key should gain a canonical resource
type/group rather than guessing equivalence across API migrations.

Approved behavior: omitted namespace remains unmatched, and supplied manifests
are assumed to target the scanned cluster. The latter assumption is surfaced in
the report itself whenever a cross-plane match occurs.

## PDB-002 multi-resource identity

PDB-002 should stop encoding two resources as
`Name: "coredns-duplicate,coredns-managed"`. The structured model should hold
two live Kubernetes references:

```text
references = [PDB kube-system/coredns-duplicate (UID A),
              PDB kube-system/coredns-managed   (UID B)]
concepts   = sort([key("k8s-object", "PodDisruptionBudget", "kube-system",
                       "coredns-duplicate"),
                   key("k8s-object", "PodDisruptionBudget", "kube-system",
                       "coredns-managed")])
fingerprint = key("finding-v2", "PDB-002", TargetVersion, concepts...)
```

Sorting makes the pair order-independent. It remains one relational finding,
not two single-resource findings, and renderers can label it from the reference
list. This removes the synthetic-name special case instead of preserving it as
permanent debt. Migration may temporarily keep a legacy display field for JSON
compatibility, but it must not participate in v2 identity.
