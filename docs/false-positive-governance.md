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
