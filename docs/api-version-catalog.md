# API Version Catalog

The API version catalog is the foundation for `v0.11.0-api-version-catalog`.
It records reviewed Kubernetes API deprecation and removal facts in a versioned
embedded model so later rule integration can distinguish:

- API removed at the target Kubernetes version
- API deprecated but still served at the target Kubernetes version
- API supported and not deprecated
- target version outside this build's verified catalog coverage

The model, validation, and deterministic lookup were the first slice added.
The updates below cover what was built on top of that foundation: `API-001`/
`API-002` rule integration, the catalog becoming the sole authored source of
lifecycle data, and a build-wide unsupported-target-version fail-safe.

**Update — rule integration:** `API-001`/`API-002` resolve every object's
removed-version fact through `apicatalog.DefaultVersioned()`. See
`internal/rules/api_catalog.go`.

**Update — catalog authority:** the versioned catalog is now the single
authored source of lifecycle data. `apicatalog.Deprecated` — the list the
k8s and manifest collectors use to discover live/manifest objects at
deprecated GVKs — is derived from the versioned catalog at package init
(`legacyFromVersioned` in `internal/apicatalog/catalog.go`), not
independently hand-maintained. `TestVersionedCatalogCoversLegacyDeprecatedAPIs`
(`internal/apicatalog/catalog_test.go`) is the one-time migration
regression guard: it checks the versioned catalog against a frozen
snapshot of the pre-migration hand-authored inventory, so a silently
dropped or altered entry during that migration (or a future accidental
edit) fails CI. `internal/rules/api_catalog.go`'s resolver has no legacy
fallback left — a live/manifest object at a group/version/kind the
catalog doesn't have an entry for now surfaces as a catalog integrity
error, not a silent guess, since `apicatalog.Deprecated` (what the
collectors report against) and the versioned catalog can no longer
disagree by construction.

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

## Coverage

The catalog has full parity with the API removals KubePreflight has ever
tracked: every group/version/kind previously in the hand-authored
`apicatalog.Deprecated` ruleset (44 entries, spanning the 1.16, 1.22,
1.25, 1.26, and 1.27 Kubernetes removal waves) has a versioned catalog
entry with `deprecatedInVersion`, `source`, `reference`, and
`lastVerifiedDate` metadata the legacy list never carried. New removals
are added here going forward — a data change to `versioned_catalog.json`,
never a code change.

One entry's `deprecatedInVersion` (`extensions/v1beta1 PodSecurityPolicy`)
is a sourced inference (from its replacement `policy/v1beta1
PodSecurityPolicy`'s known availability since Kubernetes 1.10) rather than
a directly-documented deprecation announcement — no dated source could be
found for that specific removal. Its `removedInVersion` (1.16) is
independently confirmed.

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
