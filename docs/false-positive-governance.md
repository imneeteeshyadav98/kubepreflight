# False-positive governance

KubePreflight can reduce noise only when the exception is evidence-backed and
test-locked. New checks are not the goal of this policy; trustworthy decisions
are.

All accepted suppressions and severity downgrades live in
`internal/exemptions`. A registry entry is required to include:

- a stable exemption ID
- affected rule/check IDs
- human-readable rationale
- explicit evidence
- supported resource kinds/GVKs
- supported evaluation plane (`live`, `manifest`, or `both`)
- scope boundaries
- conservative fallback behavior
- positive regression coverage
- negative near-match coverage
- spoofing coverage
- live-plane versus manifest-plane scope coverage where planes differ
- documented scope and rationale

The governance test in `internal/exemptions` fails if an entry omits any of
those fields, uses an invalid plane, references a missing test, or points at a
missing documentation anchor. Registry access returns defensive copies in
deterministic ID order.

CI also runs `scripts/check-exemption-governance.sh`, which wraps
`cmd/exemptioncheck`. The check validates registry metadata, documentation
anchors, referenced tests, the audit inventory, known production callsites, and
deterministic output.

## Final governance contract

No suppression, exclusion, or severity downgrade may be added unless it has:

- an `internal/exemptions` registry entry with a stable ID
- explicit evidence fields and conservative fallback behavior
- positive coverage proving the intended false positive is controlled
- negative near-match coverage proving similar user-owned resources still fire
- spoofing regression coverage proving names, namespaces, labels, or managers
  are not trusted outside the documented scope
- live-plane versus manifest-plane coverage when the behavior differs by plane
- a documentation anchor explaining the safety rationale and scope
- an audit inventory row tying the decision to production callsites

If any field is uncertain, the fallback is to report the finding normally. The
registry is allowed to reduce noise only after ownership or provider evidence
has been proven; it is not a place for convenience ignores.

## Exemption table

| Registry ID | Rule(s) | Plane | Behavior | Required evidence | Conservative fallback |
|---|---|---|---|---|---|
| `api001-live-events` | `API-001` | live | Suppressed entirely | Live `events.k8s.io/v1beta1 Event` object emitted by controllers or clients; manifest Events are out of scope | Report any non-live or non-Event deprecated object normally |
| `api001-auto-managed-flowcontrol` | `API-001` | live | Downgraded from Blocker to Info | `FlowSchema` or `PriorityLevelConfiguration` with kube-apiserver `apf.kubernetes.io/autoupdate-spec` evidence or EKS `eks-internal` field-manager evidence scoped to flowcontrol GVKs | Report as Blocker when evidence is missing, ambiguous, or attached to another GVK |
| `api001-controller-managed-endpointslices` | `API-001` | live | Downgraded from Blocker to Info | `EndpointSlice` with the exact built-in managed-by label and a controller `ownerReference` to a `Service` | Report as Blocker when either controller signal is missing or attached to a different kind |
| `addon-provider-scoped-catalog` | `ADDON-001`, `ADDON-002` | live | Withholds EKS-scoped compatibility lookup until provider plane is confirmed | AWS/EKS enrichment has confirmed the provider plane before applying EKS-scoped workload add-on compatibility facts | Report provider-scoped workload compatibility as unknown/unverifiable rather than borrowing provider facts |

## Accepted exemptions

### API-001 live Events <span id="api-001-live-events"></span>

Live `events.k8s.io` Event objects are suppressed entirely for `API-001`.
Events are emitted by clients and controllers, self-expire quickly, and can
exist in high volume. A per-Event Blocker is not an actionable migration task.

Scope is deliberately narrow: this applies only to live Kubernetes collection.
Manifest-plane Event YAML is user-authored input and still reports as a
Blocker.

Regression coverage includes a positive live suppression test, a manifest-plane
negative test, and guards proving unrelated deprecated objects still fire.

### API-001 auto-managed flowcontrol <span id="api-001-auto-managed-flowcontrol"></span>

Kube-apiserver and EKS-managed `FlowSchema` and
`PriorityLevelConfiguration` defaults are downgraded from Blocker to Info when
live-object evidence shows that the control plane reconciles them.

Accepted evidence is one of:

- `apf.kubernetes.io/autoupdate-spec: "true"` on kube-apiserver bootstrap
  defaults
- EKS's `eks-internal` field manager on
  `flowcontrol.apiserver.k8s.io` objects

The field-manager evidence is GVK-scoped. It does not exempt arbitrary objects
with the same manager string. Name-only spoofing is also rejected: an object
named like an EKS default remains a Blocker without controller evidence.
Manifest-plane flowcontrol YAML is never downgraded.

### API-001 controller-managed EndpointSlices <span id="api-001-controller-managed-endpointslices"></span>

Controller-managed `EndpointSlice` objects are downgraded from Blocker to Info
when they carry both built-in controller signals:
`endpointslice.kubernetes.io/managed-by: endpointslice-controller.k8s.io` and
a controller `ownerReference` to a `Service`.

Namespace or name alone is not trusted. In particular, a
`default/kubernetes`-shaped EndpointSlice without both controller evidence
signals still reports as a Blocker because KubePreflight cannot prove ownership
safely. Manifest-plane EndpointSlice YAML is user-authored and always remains a
Blocker.

### Add-on provider-scoped catalog <span id="add-on-provider-scoped-catalog"></span>

Provider-scoped add-on compatibility facts are applied only after the matching
provider plane has been confirmed. AWS Load Balancer Controller uses EKS-scoped
catalog data and therefore requires AWS/EKS enrichment. Generic Kubernetes
add-ons such as metrics-server, ingress-nginx, cert-manager, and external-dns
can use generic Kubernetes catalog entries without AWS.

If a cluster-only scan sees an AWS Load Balancer Controller-shaped workload, it
must not silently borrow EKS compatibility facts. The conservative fallback is
an ADDON-002 unknown/unverifiable warning, not an ADDON-001 incompatibility or a
no-finding compatible result.

## QUALITY-001B audit inventory

The machine-readable audit inventory lives in `internal/exemptions/audit.go`.
It records governed exemptions and reviewed paths that intentionally remain
outside the registry.

Governed in the registry:

- API-001 live Events
- API-001 auto-managed FlowSchema/PriorityLevelConfiguration defaults
- API-001 controller-managed EndpointSlices
- ADDON-001/ADDON-002 provider-scoped live workload add-on catalog lookup

Reviewed and intentionally not registry-governed:

- Add-on image repository classification: classification input, not a
  suppression or severity downgrade.
- DRAIN-001 singleton workload scope and terminal/deleting pod evidence hygiene:
  rule scope, not a trusted safe result.
- DRAIN-003 hard scheduling constraint checks: normal rule conditions that fire
  only when current node inventory proves no spare target.
- DRAIN-004 DaemonSet/control-plane/incomplete-request capacity model
  boundaries: estimate hygiene, not a managed-object exemption.
- Provider/AWS graceful degradation: coverage behavior, not proof that a
  provider-specific resource is safe.
- Namespace allowlist filtering: explicit user reporting scope, not an
  evidence-backed safety exemption.

## Contributor process

When a future change touches a path that suppresses, excludes, ignores, skips,
downgrades, or withholds a finding, treat it as a governance change first and a
code change second.

For a governed exemption:

1. Add or update the `internal/exemptions` registry entry.
2. Add positive, negative near-match, spoofing, and plane-specific tests.
3. Add or update the `internal/exemptions/audit.go` row for the production
   callsite.
4. Link the registry entry to a documentation anchor in this file.
5. Run `go run ./cmd/exemptioncheck` and
   `./scripts/check-exemption-governance.sh`.

For a reviewed path that is intentionally not governed, add or update the audit
inventory row and explain why it is rule scope, classification input, coverage
behavior, or explicit user filtering rather than a false-positive exemption.

## Reporting unsafe suppression

Report a false positive or unsafe suppression with:

- the KubePreflight version or commit
- command and flags used, including `--manifests-only`, provider flags, and
  namespace filters
- sanitized finding JSON or terminal output
- the affected resource GVK, namespace, name, and evaluation plane
- why the resource appears user-owned, provider-owned, controller-managed, or
  ambiguous
- any relevant labels, annotations, `ownerReferences`, field managers, provider
  snapshot status, and manifest source path

When ownership is ambiguous, maintainers should prefer a visible Blocker or
Warning over a hidden or downgraded finding until the evidence model is strong
enough to govern.

## CI enforcement and limits

The CI governance gate enforces the registry shape, documentation anchors,
referenced tests, audit inventory completeness, deterministic checker output,
and production registry-ID references from inventoried callsites.

The static checker is deliberately bounded. It does not prove semantic safety
for arbitrary code, infer every possible synonym for suppression, or replace
review of rule behavior. The maintained audit inventory is the source of truth
for reviewed suppression-like paths, while broad repository searches remain a
clean-room audit aid before finalizing a governance PR.

## Completion evidence

`QUALITY-001E` closes the false-positive governance phase when a clean checkout
shows:

- `go run ./cmd/exemptioncheck` passes
- `./scripts/check-exemption-governance.sh` passes
- repository search for suppression-like terms has been reconciled against the
  registry and audit inventory
- the accepted exemption table above matches `internal/exemptions/registry.go`
- intentionally separate and non-exemption paths are documented in
  `internal/exemptions/audit.go`

At that point:

```text
QUALITY-001A: COMPLETE
QUALITY-001B: COMPLETE
QUALITY-001C: COMPLETE
QUALITY-001D: COMPLETE
QUALITY-001E: COMPLETE

KP-QUALITY-001: COMPLETE
```
