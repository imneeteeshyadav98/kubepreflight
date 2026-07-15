# API Version Catalog

The API version catalog is the foundation for `v0.11.0-api-version-catalog`.
It records reviewed Kubernetes API deprecation and removal facts in a versioned
embedded model so later rule integration can distinguish:

- API removed at the target Kubernetes version
- API deprecated but still served at the target Kubernetes version
- API supported and not deprecated
- target version outside this build's verified catalog coverage

This first slice adds the model, embedded seed catalog, validation, deterministic
lookup, and source metadata. It does not change `API-001` or `API-002` behavior
yet; the rules still read the existing `Deprecated` ruleset until the integration
slice wires them into this catalog.

**Update — rule integration:** `API-001`/`API-002` now resolve each object's
removed-version fact through `apicatalog.DefaultVersioned()` first, falling
back to the legacy static `Deprecated` list whenever the versioned catalog
has no entry for that group/version/kind, or the target falls outside that
entry's own `supportedTargetRange`. See `internal/rules/api_catalog.go`.

**Update — unsupported target-version fail-safe:** the catalog document now
also declares a top-level `buildSupportedTargetRange` — the Kubernetes
target-version range this build's maintainer has explicitly verified the
embedded data against, independent of any single entry's own range. `scan`
and `plan` both reject a `--target-version`/`--to-version` outside this
range before any kubeconfig loading, collection, or report/action-plan
generation, via `VersionedCatalog.TargetSupported` and
`internal/cli`'s `rejectUnsupportedTargetVersion`. This is a coarser,
build-wide gate, distinct from a per-API entry's own range check used by
the rule integration above.

## Model

Each catalog entry includes:

- API group, version, resource, kind, and scope
- deprecated Kubernetes version
- removed Kubernetes version
- replacement API and optional replacement API version
- supported target-version range for this catalog entry
- source and reference
- last verified date
- confidence

The current schema version is:

```text
apicatalog.kubepreflight.io/v1
```

## Initial Coverage

The seed catalog starts with representative high-signal APIs from the existing
`internal/apicatalog.Deprecated` ruleset:

- PodSecurityPolicy
- PodDisruptionBudget
- CronJob
- HorizontalPodAutoscaler
- Ingress
- FlowSchema
- PriorityLevelConfiguration

Full coverage parity with the legacy ruleset is intentionally left to the
catalog maintenance slice, where coverage/staleness checks can become hard CI
gates without changing detection behavior in the same PR.

## Validation Rules

Catalog loading rejects:

- unsupported schema versions
- malformed JSON
- missing API version, resource, kind, replacement, source, reference,
  confidence, or verified date
- malformed deprecated, removed, or supported target versions
- deprecated versions after removed versions
- supported target ranges whose minimum is after their maximum
- duplicate or overlapping entries for the same group/version/kind
- a missing, malformed, or inverted top-level `buildSupportedTargetRange`

Lookup normalizes API group, API version, and target patch versions. Missing
entries remain unknown; callers must not infer compatibility from absence.

## Source Policy

Catalog entries must be reviewed before they affect findings. The initial source
is the Kubernetes Deprecated API Migration Guide. Do not automatically scrape
external sources into the production catalog without human review. Every entry
must keep source attribution and a last verified date.
