# Compatibility Catalog

The compatibility catalog is the foundation for `v0.10.0-compatibility-catalog`.
It records reviewed add-on compatibility facts in a small versioned model so
later rules can distinguish:

- known compatible
- known incompatible
- compatible but upgrade recommended
- unknown or unverifiable
- the operational impact area used by context-aware add-on upgrade gating

The first slice added the model, embedded seed catalog, validation,
deterministic lookup, and version-status helpers, and wired EKS managed
add-ons into it. A second slice extended the same catalog to live workload
add-ons discovered from cluster state directly: metrics-server, ingress-nginx,
AWS Load Balancer Controller, cert-manager, and external-dns. The catalog also
records operational-impact metadata for each entry. `ADDON-001` uses that
metadata with the selected `--upgrade-context` to decide whether a confirmed
incompatibility blocks the selected operation, requires operator decision, or
is visible audit-only evidence.

For any add-on covered by the catalog (EKS managed or live workload):

- installed versions below the known minimum produce `ADDON-001` findings;
  severity and upgrade gate are context-aware
- parseable versions that meet the minimum but are below the recommendation
  produce `ADDON-002` Warning findings
- known compatible recommended versions produce no compatibility finding
- missing catalog targets, malformed versions, custom builds, and other
  unknowns remain `ADDON-002` Warning findings

A given workload never produces both an `ADDON-001` and an `ADDON-002`
finding: `ADDON-001` owns the known-incompatible status exclusively,
`ADDON-002` explicitly skips it.

### Live workload add-ons

Live workload add-ons are identified from Deployment/DaemonSet container
images, not names or labels — `registry.k8s.io/ingress-nginx/controller`
identifies ingress-nginx regardless of what the Deployment or its namespace
happens to be named, and a workload whose name merely mentions an add-on
(e.g. a test harness called `my-ingress-nginx-test`) never matches without
the real image. This is also what keeps cert-manager's controller,
cainjector, and webhook Deployments from being conflated: each ships under a
distinct image repository, and only the controller's is catalog-tracked.

AWS Load Balancer Controller is catalogued under the `eks` provider (it only
makes sense on EKS); its catalog entry is only applied when the scan has
confirmed AWS/EKS enrichment (`--provider=eks`) — a cluster-only scan that
happens to have an ALB Controller-shaped Deployment installed falls back to
the ordinary unverifiable warning instead of trusting EKS-specific
compatibility data on an unconfirmed cluster type. Ingress controllers
without a catalog entry (traefik, haproxy-ingress, kong-ingress) keep the
original conservative unverifiable-warning behavior.

## Model

Each catalog entry includes:

- Kubernetes target version
- provider
- add-on name
- minimum compatible version
- recommended version
- operational impacts
- source
- reference
- last verified date
- confidence

The current schema version is:

```text
compatcatalog.kubepreflight.io/v1
```

### Operational impacts

`operationalImpacts` is a string list describing which part of an operation an
add-on compatibility finding may affect. It supports multiple values because an
add-on can participate in more than one operational path. For example, VPC CNI
is both networking/data-plane infrastructure and relevant to worker rollouts.

Supported values are:

| Value | Meaning |
| --- | --- |
| `control_plane_dependency` | Compatibility may matter to control-plane upgrade prerequisites or provider-managed control-plane validation. |
| `worker_rollout_dependency` | Compatibility may matter when workers are cordoned, drained, replaced, or rolled. |
| `networking_data_plane` | The add-on participates in pod/service/networking data-plane behavior. |
| `storage_data_plane` | The add-on participates in persistent storage attach/mount/provisioning behavior. |
| `cluster_dns` | The add-on participates in cluster DNS behavior. |
| `admission_api_dependency` | The add-on participates in admission, API extension, or API write paths. |
| `workload_dependency` | Application scheduling, startup, traffic, storage, DNS, or certificate behavior may depend on it. |
| `optional_ecosystem` | The add-on is useful ecosystem software but is not proven by catalog data alone to block every upgrade operation. |
| `operator_review` | The correct gate depends on local install, health, rollout, or upgrade-plan evidence not present in the catalog entry. |
| `unknown` | The impact is intentionally unknown. Missing or empty metadata normalizes to this value and must not be treated as safe. |

`unknown` must stand alone; do not combine it with other impacts. Use
`operator_review` with concrete impacts when the impacted area is known but the
gate still depends on environment or sequencing evidence.

Backward compatibility: the field is additive. Entries that omit
`operationalImpacts`, or provide an empty value, are accepted and normalized to
`["unknown"]`. This keeps older custom catalogs loadable while making the lack
of operational-impact evidence explicit. Unknown impact never becomes a
confirmed `ADDON-001` blocker by itself; it requires operator decision outside
audit-only scans.

## Initial Coverage

The seed catalog contains reviewed entries for:

EKS managed add-ons (provider `eks`):

- Amazon VPC CNI
- kube-proxy
- CoreDNS
- EBS CSI driver
- EFS CSI driver
- AWS Load Balancer Controller

Live workload add-ons (provider `kubernetes`, applies to any cluster type):

- metrics-server
- ingress-nginx
- cert-manager
- external-dns

## Validation Rules

Catalog loading rejects:

- unsupported schema versions
- malformed JSON
- missing provider, add-on, source, reference, confidence, or verified date
- malformed Kubernetes target versions
- unparseable minimum or recommended versions
- minimum versions greater than recommended versions
- unsupported, duplicated, or invalidly-combined operational impacts
- duplicate entries for the same provider, add-on, and Kubernetes target
- a known add-on (any entry in `RequiredAddons`) filed under the wrong
  provider — e.g. cert-manager, which is ordinary cluster-agnostic software,
  entered under provider `eks` instead of `kubernetes`

Lookup normalizes provider, add-on name, and Kubernetes patch versions. Missing
entries remain unknown; callers must not infer compatibility from absence.

## Validation Command

```bash
scripts/check-compatibility-catalog.sh
scripts/check-compatibility-catalog.sh --stale-after-days 90
```

Wired into CI's `verify` job (`.github/workflows/ci.yml`) — a broken or
incomplete catalog entry fails CI before merge, not silently at runtime as
every affected add-on quietly falling back to "unverifiable". It:

1. Loads and validates the embedded catalog (all rules above), exiting 1 on
   failure.
2. Prints the full catalog as a stable `Provider | Add-on | Kubernetes
   target | Minimum | Recommended | Verified` matrix, in the same
   deterministic order every run — useful for review and release notes.
3. Checks required coverage (see below), exiting 1 if any catalog-supported
   target version is missing a required add-on.
4. Reports (never fails on) entries verified more than `--stale-after-days`
   ago (default 180).

The same required-coverage check also runs as a normal Go test
(`TestDefaultCatalogHasFullRequiredCoverage` in
`internal/compatcatalog/catalog_test.go`, via the same `MissingRequiredEntries`
method), so a coverage gap fails `go test ./...` too, not just the
standalone script.

### Required coverage matrix

`RequiredAddons` (`internal/compatcatalog/catalog.go`) is the fixed list of
add-ons the catalog must cover for **every** Kubernetes target version it
models — computed from whatever target versions actually appear in the
catalog (`Catalog.TargetVersions()`), not a hardcoded version list, so
adding a new target version's entries automatically extends what's checked:

| Add-on | Required provider |
| --- | --- |
| vpc-cni | `eks` |
| kube-proxy | `eks` |
| coredns | `eks` |
| aws-ebs-csi-driver | `eks` |
| aws-efs-csi-driver | `eks` |
| aws-load-balancer-controller | `eks` |
| metrics-server | `kubernetes` |
| ingress-nginx | `kubernetes` |
| cert-manager | `kubernetes` |
| external-dns | `kubernetes` |

A provider-specific add-on (currently only `aws-load-balancer-controller`)
is only required under its own provider — the check never expects an
`eks`-scoped add-on to also have a `kubernetes`-scoped entry, or vice versa.

## Maintenance: Adding a New Kubernetes Release

Follow this process in order when the catalog needs entries for a new target
Kubernetes version:

1. Identify the supported target Kubernetes version (`major.minor`, e.g.
   `1.35`).
2. Collect official source references for each add-on — the project's own
   published compatibility matrix or release notes, not a third party's
   summary of them.
3. Verify the minimum compatible version against that source.
4. Verify the recommended version against that source (may be the same as
   the minimum if the source doesn't distinguish the two).
5. Record operational impacts. Use more than one value when the add-on clearly
   spans multiple operational paths; use `operator_review` when the impacted
   area is known but the selected operation still needs local evidence; use
   `unknown` only when the impact cannot be supported from repository evidence
   or reviewed source material.
6. Record the correct provider and add-on name — `RequiredAddons`
   (`internal/compatcatalog/catalog.go`) is the canonical list of known
   add-on names and their required provider; a mismatch is a validation
   error, not a silent typo.
7. Record an accurate verification date (`YYYY-MM-DD`, the date you
   actually checked the source) and an honest confidence tier:
   - `PROVIDER_REPORTED` — the source is the provider's own API/documented
     compatibility data (e.g. AWS's `DescribeAddonVersions`, AWS's EKS add-on
     docs).
   - `STATIC_CERTAIN` — a project's own published, authoritative version
     support table (e.g. ingress-nginx's, cert-manager's).
   - `OBSERVED` — inferred from release notes or general project
     conventions without a single authoritative compatibility table (e.g.
     external-dns).
8. Add the catalog entry to `internal/compatcatalog/catalog.json`.
9. Run `scripts/check-compatibility-catalog.sh` and fix anything it flags.
10. Add compatible/incompatible/unknown test cases for the new entry (see
   `internal/rules/addon001_test.go` for the established pattern — one test
   per `compatcatalog.Status` outcome, using the same realistic image/version
   fixtures the existing tests use).
11. Run the full detector regression suite (`go test ./...`) before opening
    a PR — a catalog change is a behavior change for every scan that hits
    the affected add-on/target combination, not just an isolated data edit.

## Maintenance Policies

- **No unreviewed automated scraping.** Every entry is added by a human
  reading the source and writing the entry by hand (or reviewing a
  proposed entry line by line) — never an automated pipeline that ingests
  a scraped page directly into `catalog.json`.
- **Official sources preferred.** A provider's own documentation or API,
  or a project's own published compatibility table, over a blog post,
  forum answer, or another tool's redistributed data.
- **Unknown must remain conservative.** A missing catalog entry, an
  unparseable installed version, or a provider scope the current scan
  hasn't confirmed (see AWS Load Balancer Controller above) must produce
  an `ADDON-002` unverifiable warning, never a guessed Blocker or a
  silently-skipped "must be fine."
- **Missing impact metadata is not safe.** Omitted or empty
  `operationalImpacts` values normalize to `unknown`. Context-aware gating
  treats that as requiring operator-aware handling, not as permission to
  allow an operation or as proof of a confirmed blocker.
- **Catalog updates require tests.** A `catalog.json` change without a
  corresponding test change should be treated as incomplete review, not a
  quick data fix — see step 9 above.
- **Old target entries are never silently overwritten.** Adding entries
  for a new Kubernetes target version must not modify or remove an
  existing target version's entries in the same change; if an existing
  entry is genuinely wrong, fix it as its own explicit, reviewed change
  with an updated verification date, not a side effect of an unrelated
  addition.
- **Changing a minimum version is a product behavior change.** Raising or
  lowering `minimumCompatibleVersion` for an existing entry changes which
  real installed versions become `ADDON-001` findings and may become
  blockers in matching contexts — treat it with the same scrutiny as a
  rule-severity change, including a clear explanation in the PR description
  of what source justifies the new number.

## Source Policy

Catalog entries must be reviewed before they affect findings. Do not
automatically scrape external sources into the production catalog without human
review. Every entry must keep source attribution and a last verified date.
