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

## Source-of-truth JSON and the generated Go view

`internal/apicatalog/versioned_catalog.json` is the single authored source
of lifecycle data. `apicatalog.Deprecated` — the legacy-shaped
`[]DeprecatedAPI` slice the k8s and manifest collectors use to discover
live/manifest objects at deprecated GVKs — is a **derived, generated
view** of that same JSON, computed once at package init
(`legacyFromVersioned` in `internal/apicatalog/catalog.go`). It is never
edited directly, and there is no code path that could make it disagree
with the versioned catalog: the two are the same data, viewed two ways.

`internal/rules/api_catalog.go`'s resolver relies on this: a live or
manifest object reported at some group/version/kind is guaranteed to have
a matching versioned catalog entry, because the collector only ever
discovers objects at GVKs it read from `Deprecated`, which is itself
generated from the versioned catalog. A lookup miss there is therefore
always a catalog **integrity error** (the two sides have somehow drifted
apart — a bug), never a legitimate "unknown API" the rule should quietly
skip.

## Validation Command

```bash
scripts/check-api-version-catalog.sh
scripts/check-api-version-catalog.sh --stale-after-days 90
```

Wired into CI's `verify` job in both `.github/workflows/ci.yml` and
`.github/workflows/release.yml` — a broken, incomplete, or drifted
catalog entry fails CI before merge or release, not silently at scan
time. It:

1. Loads and validates the embedded catalog (every rule in Validation
   Rules above), exiting 1 on failure.
2. Prints the full catalog as a stable `Group | Version | Kind |
   Deprecated | Removed | Supported target range | Verified` table, in
   the same deterministic order every run.
3. Checks frozen legacy inventory parity (`apicatalog.LegacyParityIssues`)
   — every group/version/kind that was ever in the pre-migration
   hand-authored list must still be present with matching fields, exiting
   1 on any drop, addition, or mismatch.
4. Checks that the declared `buildSupportedTargetRange` is fully and
   exactly covered — every minor version inside it reports supported, and
   the versions immediately outside it (one below, one above) correctly
   report unsupported — exiting 1 on a gap.
5. Reports (never fails on) entries verified more than
   `--stale-after-days` ago (default 180).
6. Runs the whole check **twice** and diffs the two runs' output —
   exiting 1 if they aren't byte-identical. The catalog's own sort is
   already unit-tested as deterministic; this is the end-to-end version
   of that guarantee, run through the same binary CI actually invokes.

The same parity, coverage, and determinism checks also run as ordinary Go
tests (`TestVersionedCatalogCoversLegacyDeprecatedAPIs`,
`TestDefaultVersioned_SupportsFullDeclaredTargetRange`,
`TestDefaultVersioned_DeterministicAcrossCalls` in
`internal/apicatalog/`), so a regression fails `go test ./...` too, not
just the standalone script.

## Maintenance: Adding a New Kubernetes Release

Follow this process in order when the catalog needs entries for a newly
announced Kubernetes API removal wave:

1. Identify the exact Kubernetes minor version the removal takes effect
   in, and every group/version/kind it affects.
2. Collect authoritative source references, in priority order:
   1. The official Kubernetes Deprecated API Migration Guide
      (`kubernetes.io/docs/reference/using-api/deprecation-guide/`).
   2. An official `kubernetes.io` blog post specifically about that
      release's API removals.
   3. The upstream `kubernetes/api` (or the relevant sibling repo, e.g.
      `kubernetes/apiextensions-apiserver`, `kubernetes/kube-aggregator`)
      `zz_generated.prerelease-lifecycle.go` source file for the type —
      this is the actual generator input the official deprecation guide
      itself is built from, and the most authoritative source available
      when the guide's prose doesn't cover an older removal wave.
   A third party's redistributed summary (blog aggregators, unofficial
   changelogs) is never an acceptable source on its own.
3. Verify `deprecatedInVersion` and `removedInVersion` against that
   source. If no dated source can be found for `deprecatedInVersion`
   specifically (this has happened once — see the `extensions/v1beta1
   PodSecurityPolicy` entry), a reasoned inference is acceptable, but
   **must say so directly in the entry's own `source` field** — never
   silently presented with the same confidence as a directly-documented
   fact. `removedInVersion` (what actually drives `API-001`/`API-002`
   firing) should not need this caveat; if it does, treat that as
   blocking, not just a note.
4. Set `replacementAPI` (and `replacementAPIVersion`, when it's a direct
   apiVersion swap) from the same source.
5. Set `supportedTargetRange` for the new entry: `min` is normally the
   new `removedInVersion`, `max` matches the catalog's current
   `buildSupportedTargetRange.max` (do not extend an individual entry's
   range further than the build's own declared coverage).
6. Record an accurate `lastVerifiedDate` (`YYYY-MM-DD`, the date you
   actually checked the source) and `confidence: "STATIC_CERTAIN"` — the
   only tier this schema currently supports; if a fact genuinely can't be
   asserted with that level of certainty, it isn't ready to add yet in
   this schema.
7. Add the entry to `internal/apicatalog/versioned_catalog.json`.
8. Run `scripts/check-api-version-catalog.sh` and fix anything it flags.
9. Add API-001 (removed at target) and API-002 (deprecated but still
   served) test cases for the new entry — see
   `internal/rules/api_catalog_test.go` for the established pattern.
10. Run the full detector regression suite (`go test ./...`) before
    opening a PR — a catalog change is a behavior change for every scan
    that hits the affected GVK/target combination, not just an isolated
    data edit.

### When to raise `buildSupportedTargetRange.max`

This is a deliberate, one-way gate — `internal/cli`'s
`rejectUnsupportedTargetVersion` uses it to refuse a scan against a
target this build was never actually checked against (see PR 3's
unsupported-target fail-safe). Raise it only when true:

```text
Catalog fully verified through 1.39
→ target 1.40 rejected

1.40's lifecycle data is complete and every affected GVK is reviewed
→ buildSupportedTargetRange.max updated to 1.40
```

Never bump `max` speculatively "to unblock a user" — that defeats the
entire point of the fail-safe, which is refusing to guess about a target
version whose removal facts haven't actually been checked.

## Maintenance Policies

- **No unreviewed automated scraping.** Every entry is added by a human
  reading the source and writing the entry by hand (or reviewing a
  proposed entry line by line) — never an automated pipeline that ingests
  a scraped page directly into `versioned_catalog.json`.
- **Official sources preferred**, in the priority order listed above.
- **Uncertain historical data must be visible, not smoothed over.** A
  `deprecatedInVersion` that's an inference rather than a directly-sourced
  fact must say so in that entry's own `source` field — the schema has no
  separate lower-confidence tier, so the text itself is what keeps this
  honest for the next reader.
- **Lifecycle data changes require tests.** A `versioned_catalog.json`
  change without a corresponding `internal/rules/api_catalog_test.go` (or
  `internal/apicatalog`) test change should be treated as incomplete
  review, not a quick data fix — see step 9 above.
- **Old entries are never silently overwritten.** Adding entries for a
  newly announced removal wave must not modify or remove an existing
  entry in the same change; if an existing entry is genuinely wrong, fix
  it as its own explicit, reviewed change with an updated verification
  date, not a side effect of an unrelated addition.

## Source Policy

Catalog entries must be reviewed before they affect findings. The initial source
is the Kubernetes Deprecated API Migration Guide. Do not automatically scrape
external sources into the production catalog without human review. Every entry
must keep source attribution and a last verified date.
