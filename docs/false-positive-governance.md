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
when they carry the built-in controller label:
`endpointslice.kubernetes.io/managed-by: endpointslice-controller.k8s.io`.

Namespace or name alone is not trusted. In particular, a
`default/kubernetes`-shaped EndpointSlice without the managed-by evidence still
reports as a Blocker because KubePreflight cannot prove ownership safely.
Manifest-plane EndpointSlice YAML is user-authored and always remains a
Blocker.

Existing add-on, workload ownership, provider-managed, and other classification
paths are intentionally left for the follow-up migration audit in
`QUALITY-001B`; this first slice governs only the API-001 exemptions touched by
the initial implementation.
