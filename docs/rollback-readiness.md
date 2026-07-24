# Rollback Readiness

Status: planned for `v0.12.0`.

KubePreflight does not replace Amazon EKS rollback operations or AWS-native
rollback readiness insights. The rollback workflow is assessment-only: it
combines AWS eligibility evidence, AWS rollback/upgrade insights, Kubernetes
live evidence, and existing KubePreflight checks to help an operator decide
whether rollback is available, operationally ready, and preferable to
fix-forward.

The tool must not invoke rollback, update node groups, downgrade add-ons, call
`--force`, modify PDBs, or remove disruption annotations.

## Assessment Modes

`rollback plan` is the pre-upgrade posture assessment. It answers whether an
operational rollback path is likely to remain open if the upgrade introduces a
regression.

`rollback assess` is the post-upgrade readiness assessment. It evaluates the
current cluster state against EKS rollback eligibility, evidence freshness, and
operational risk before an operator decides whether to roll back or fix forward.

## Decision Layers

Rollback output keeps three decisions separate so the report cannot contradict
itself.

Eligibility:

- `eligible`
- `unavailable`
- `unknown`

Readiness:

- `ready`
- `blocked`
- `high_risk`
- `insufficient_evidence`

Recommendation:

- `rollback_preferred`
- `fix_forward_preferred`
- `operator_decision_required`
- `do_not_proceed`

KubePreflight should avoid saying rollback is "safe". Preferred wording is that
rollback is eligible and no blocking risks were detected based on currently
available evidence.

## Schema

Rollback assessments use:

```text
kubepreflight.io/rollback-assessment/v1alpha1
```

The initial model records:

- cluster identity and current/rollback target versions
- eligibility status, rollback window, source, and reason codes
- readiness status with blocker, warning, and unknown counts
- recommendation decision, confidence, and reason codes
- evidence freshness and completeness
- per-check status, evidence, and reason codes

Reason codes are deterministic constants rather than free-form classifier
output. This keeps the first implementation rules-based and reviewable.

## EKS Eligibility Evidence

The initial EKS eligibility slice is read-only and uses AWS APIs to collect:

- cluster version, region, support type, and status
- cluster update history through `ListUpdates` and `DescribeUpdate`
- EKS-supported Kubernetes versions through `DescribeClusterVersions`
- observed-at timestamps and per-operation collection errors

Collection failures do not mean rollback is unavailable. Missing permissions,
timeouts, or unavailable update-history calls produce:

```text
eligibility: unknown
readiness: insufficient_evidence
reason: EKS_UPGRADE_HISTORY_UNAVAILABLE
```

Confirmed hard prerequisites produce `eligibility: unavailable`, for example:

- rollback window expired
- cluster status is not `ACTIVE`
- rollback target version is not supported by EKS
- previous version cannot be identified as exactly `N-1`

The eligibility evaluator does not decide that rollback is preferred. It only
establishes whether the rollback path is known to be available, unavailable, or
unknown. Later slices add EKS insight normalization, operational readiness, and
fix-forward versus rollback recommendations.

## EKS Rollback Readiness Insights

KubePreflight collects EKS rollback readiness insights with:

```text
category: ROLLBACK_READINESS
```

The collector reads all `ListInsights` pages, then calls `DescribeInsight` for
each returned insight so reports can preserve AWS insight IDs, names,
descriptions, recommendations, affected resources, `lastRefreshTime`, and
`lastTransitionTime`.

Insight status mapping:

- `PASSING` -> no detected AWS rollback issue for that check
- `WARNING` -> advisory risk, readiness becomes `high_risk`
- `ERROR` -> blocking risk, readiness becomes `blocked`
- `UNKNOWN` -> blocking/incomplete AWS evidence, readiness becomes `blocked`

Insight collection errors are not treated as rollback unavailability. They
produce:

```text
readiness: insufficient_evidence
reason: EKS_INSIGHTS_UNAVAILABLE
```

Rollback insights are point-in-time evidence. If an insight has no refresh time
or its `lastRefreshTime` is older than the 24-hour freshness window, the
assessment records:

```text
readiness: insufficient_evidence
reason: EKS_INSIGHTS_STALE
```

KubePreflight does not automatically call `StartInsightsRefresh`; refresh is an
operator action and can be added later behind an explicit flag.

## Operational Readiness Evidence

Operational readiness reuses evidence KubePreflight already collected for a
normal scan instead of adding a new mutating workflow. It evaluates:

- EKS managed node group versions and health context
- self-managed and hybrid node evidence availability
- Fargate evidence availability and Fargate-specific findings when present
- EKS managed add-on compatibility inventory and add-on findings
- self-managed add-on compatibility warnings
- unhealthy workload findings
- PDB and drain-readiness findings
- API, CRD, and webhook reverse-compatibility findings
- Kubernetes, AWS, and manifest coverage completeness

Readiness outcomes are still separated from recommendation:

- blocking operational findings -> `readiness: blocked`
- warnings such as newer managed node groups or unhealthy workloads ->
  `readiness: high_risk`
- missing evidence such as partial coverage -> `readiness: insufficient_evidence`
- no observed risks and complete evidence -> `readiness: ready`

This slice does not choose rollback versus fix-forward. It only updates
readiness and appends deterministic checks/reason codes. Recommendation decisions
remain the responsibility of the later deterministic decision-engine slice.

### Finding Ingestion Semantics

The optional `--findings` input is consumed as provided. Today rollback
operational readiness maps most normal scan findings by rule ID family and raw
`severity`:

- raw `Blocker` -> operational check `fail`
- raw `Warning` or `Info` -> operational check `warning`

PDB and drain-disruption findings are more conservative. A forward scan may mark
PDB or drain readiness as blocking for a worker rollout, but rollback does not
always drain nodes, evict pods, restart workloads, or replace worker capacity.
For that family, rollback readiness treats impact scopes as relevance metadata,
not proof that the rollback operation will activate the disruption path.

Default PDB/drain routing:

- `PDB-*` and `DRAIN-*` blockers -> operational check `warning` unless rollback
  disruption activation evidence is confirmed
- `PDB-*` and `DRAIN-*` warnings -> operational check `warning`
- `DRAIN-005` -> workload-health only, because it represents current workload
  health rather than disruption readiness

This is the current compatibility contract, not the final rollback semantics
model. Rollback readiness does not yet consume:

- `upgradeGate` / effective upgrade gate
- `upgradeContext`
- compatibility-catalog `operationalImpacts`
- automatic compatibility recalculation against the rollback target version

`upgradeGate` is a forward-operation concept. A finding that allows the selected
forward operation may still be relevant rollback evidence, and a finding that
blocks a forward worker rollout may not apply to an EKS control-plane rollback.
The current implementation only distinguishes the PDB/drain disruption family;
API, CRD, webhook, and add-on routing still consume provided finding severity.

### API evidence target validation

API-001 and API-002 are target-version-specific rules: their raw severity is
computed against whatever `targetVersion` the supplied `findings.json` was
generated for (`internal/rules/api001.go`'s `targetReachesRemoval` and
`internal/rules/api002.go`'s `targetBeforeRemoval`), not against the actual
rollback target. Rollback operational readiness validates the supplied
findings' target provenance against `Cluster.RollbackTargetVersion` before
trusting API-001/API-002 severity as rollback evidence:

- when `findings.json`'s `targetVersion` and the rollback target are both
  known and normalize to the same Kubernetes minor version, API-001/API-002
  routing is unchanged: a raw `Blocker` still becomes a `reverse-compatibility`
  `fail`, and a raw `Warning` still becomes a `warning`.
- when both are known but normalize to different minor versions, or when
  either is missing/unparseable, the `reverse-compatibility` check becomes
  `unknown` (insufficient evidence) instead of a confirmed `fail`/`warning`.
  The check carries reason code `ROLLBACK_EVIDENCE_TARGET_MISMATCH` (known,
  differing targets) or `ROLLBACK_EVIDENCE_TARGET_UNKNOWN` (missing or
  unparseable target), plus evidence naming the supplied findings target and
  the actual rollback target.
- this mismatch/unknown state alone does not block rollback: it feeds
  `readiness: insufficient_evidence`, not `readiness: blocked`, so it cannot
  by itself produce recommendation `do_not_proceed` or exit code 2.
- this validation only checks provenance. KubePreflight does not yet
  recalculate API compatibility findings against the actual rollback target
  from live cluster evidence -- that remains future work (see below).
- CRD-001, CRD-002, and every other rule family routed through rollback
  operational readiness are unaffected: those are current-cluster-state
  checks (see the note below) or are explicitly out of this validation's
  scope, and their routing is unchanged by this section.

### Findings cluster identity validation

`--findings` accepts any `findings.json` on disk -- nothing proves by
itself that it was generated by scanning the same cluster the rollback
assessment is evaluating. Rollback operational readiness validates the
supplied findings' cluster identity against the rollback assessment's own
live cluster identity (`Cluster.Name`/`Cluster.Region`, populated from a
live `DescribeCluster` call) before trusting cluster-specific evidence:

- **Fields used**: cluster name and region only -- the strongest identity
  fields that survive `--redact-sensitive-identifiers` unchanged (see
  "Redacted reports" below). No AWS account ID, cluster ARN, API-server
  endpoint, or Kubernetes cluster UID is collected or compared. Values are
  compared exactly after trimming whitespace on both sides -- no
  case-folding, since AWS cluster names and regions are case-sensitive by
  convention.
- **Match**: when the findings report's live EKS cluster name and region
  both match the assessed cluster's, cluster-specific evidence is consumed
  normally -- current behavior is unchanged.
- **Mismatch**: when both sides have a known cluster name and region but
  either differs, cluster-specific checks (node groups, managed add-ons,
  self-managed add-ons, workload health, PDB/drain disruption, and the
  CRD-*/WH-* portion of reverse-compatibility) become `unknown` (insufficient
  evidence) instead of a confirmed pass/fail/warning. The check carries
  reason code `ROLLBACK_EVIDENCE_CLUSTER_MISMATCH` plus evidence naming both
  clusters (name and region only). A mismatch alone feeds
  `readiness: insufficient_evidence`, never `readiness: blocked`, and cannot
  by itself produce recommendation `do_not_proceed` or exit code 2.
- **Unknown**: when the findings report is expected to carry live cluster
  identity (it isn't manifest-only) but the cluster name and/or region is
  missing or unparseable on either side -- including when the rollback
  assessment's own cluster identity is incomplete -- the same checks become
  `unknown` with reason code `ROLLBACK_EVIDENCE_CLUSTER_UNKNOWN`. This is
  equally conservative: absence of identity is never treated as a match.
- **Not applicable (manifest-only reports)**: a `findings.json` produced by
  `kubepreflight scan --manifests-only` has no live cluster evidence at all
  (no kubeconfig was ever loaded, no AWS/EKS enrichment attempted). This is
  not treated as a wrong-cluster report -- there is no live-cluster claim to
  evaluate, so no `ROLLBACK_EVIDENCE_CLUSTER_MISMATCH`/`_UNKNOWN` reason is
  emitted. Identity-independent evidence (for example, a manifest-plane
  `DRAIN-001` finding, which needs no live cluster to be valid -- see
  `internal/rules/drain001.go`) remains available; live-cluster-specific
  inventory and findings simply have nothing to consume, since none of those
  rule families run without live cluster access.
- **Redacted reports**: `--redact-sensitive-identifiers` destroys
  `EKSCluster.ARN` to a fixed `[redacted-arn]` placeholder but deliberately
  leaves `EKSCluster.ClusterName`/`.Region` untouched (see
  `internal/redact/report.go`'s design comment). Cluster identity matching
  behaves identically for redacted and unredacted reports as a result.
- **Scope**: this validates once per `ApplyOperationalReadiness` call and is
  reused by every affected check -- it is not reimplemented per check.
  `validateAPIEvidenceTarget` (the PR #207 target-version check) is
  unaffected and stays independent: API-001/API-002 routing continues to be
  gated only by target-version provenance, not cluster identity. This is a
  deliberate boundary, not an oversight -- see the next paragraph.

**Why API-001/API-002 aren't cluster-identity-gated in this PR**: a single
finding's `Resources` can carry a mix of `PlaneLive` and `PlaneManifest`
references (the "cross-plane matches" the top of this document already
mentions), and the current report model does not cleanly expose, per
API-001/API-002 finding, whether that specific finding is live-cluster-
specific evidence or manifest-derived. Rather than guess or half-implement
per-finding plane gating, this PR leaves `validateAPIEvidenceTarget` exactly
as PR #207 defined it and defers cluster-identity gating for API-001/API-002
to future work once per-finding plane provenance can be established safely.

### Findings freshness validation

A same-cluster, correct-target `findings.json` can still be hours, days, or
weeks old by the time a rollback assessment runs. Rollback operational
readiness validates the supplied findings' age -- `findings.json`'s
`scannedAt` compared against the rollback assessment's own evaluation time
(`GeneratedAt`) -- before trusting live-cluster operational evidence (node
groups, managed/self-managed add-ons, workload health, PDB/drain disruption,
and the CRD-*/WH-* portion of reverse-compatibility) as still current:

- **Threshold**: a fixed, conservative **24-hour maximum age**, computed once
  per `ApplyOperationalReadiness` call and reused by every affected check --
  it is not reimplemented per check. This is an interim, report-wide safety
  ceiling, not a claim that every evidence family is equally volatile.
  Family-specific thresholds and a user-configurable/CLI override are
  explicitly deferred to later work.
- **Fresh**: when `scannedAt`'s age (evaluation time minus `scannedAt`) is
  **24 hours or less** -- exactly 24 hours is still accepted -- affected
  checks consume live-cluster evidence normally; current behavior is
  unchanged.
- **Stale**: when the age exceeds 24 hours, affected checks become `unknown`
  (insufficient evidence) instead of a confirmed pass/fail/warning. The check
  carries reason code `ROLLBACK_EVIDENCE_STALE` plus evidence naming the
  scan time, the evaluation time, and the computed age. Stale evidence alone
  feeds `readiness: insufficient_evidence`, never `readiness: blocked`, and
  cannot by itself produce recommendation `do_not_proceed` or exit code 2 --
  even a raw `Blocker` finding in a stale report does not become a confirmed
  rollback failure.
- **Missing or invalid timestamp**: when `scannedAt` is zero/missing, or when
  the rollback assessment's own evaluation time is zero/missing, affected
  checks become `unknown` with reason code
  `ROLLBACK_EVIDENCE_TIMESTAMP_UNKNOWN`. This is equally conservative:
  absence of a trustworthy timestamp is never treated as freshness.
- **Future timestamps and clock skew**: a `scannedAt` up to **5 minutes**
  ahead of the evaluation time is tolerated and still treated as fresh (age
  is clamped to zero for evidence text, never shown as negative). A
  `scannedAt` more than 5 minutes ahead is treated the same as a missing
  timestamp -- `ROLLBACK_EVIDENCE_TIMESTAMP_UNKNOWN`, not fresh -- and the
  evidence text states that future clock skew was detected. A negative age
  is never used as proof of freshness.
- **Not applicable (manifest-only reports)**: a `findings.json` produced by
  `kubepreflight scan --manifests-only` has no live cluster evidence at all
  (see "Findings cluster identity validation" above for the shared
  manifest-only detection this freshness check reuses without duplicating
  it). Such a report is not assigned a false stale or unknown-timestamp
  state -- there is nothing live-cluster-specific for age to have gotten
  wrong -- and identity-independent manifest evidence remains available per
  current behavior.
- **Composition with identity and API target validation**: freshness
  composes with cluster identity and API-target validation on the same
  check without ever producing duplicate checks or contradictory results. A
  check stays a single canonical `Check`; every applicable reason code is
  retained (deduplicated, never repeated); status stays `unknown` whenever
  any required provenance gate fails. For example, a report that is both
  stale and cluster-mismatched resolves to one check carrying both
  `ROLLBACK_EVIDENCE_STALE` and `ROLLBACK_EVIDENCE_CLUSTER_MISMATCH`, still
  `unknown`, never fail/pass. Live-cluster evidence is only consumed as
  confirmed when every required provenance gate (identity **and**
  freshness) is in its "good" state.
- **Scope**: exactly the same six check families
  `validateClusterEvidenceIdentity` already gates -- node groups,
  managed/self-managed add-ons, workload health, PDB/drain disruption, and
  the CRD-*/WH-* portion of reverse-compatibility. API-001/API-002 target
  validation, API compatibility recalculation, eligibility, and EKS
  rollback-readiness insights (which already has its own unrelated 24-hour
  staleness gate -- see "EKS Rollback Readiness Insights" above) are
  untouched by this validation.

### Known Limitations

These limitations describe current behavior so later semantic changes can be
reviewed deliberately. They are not recommendations that every rollback is
unsafe.

- CRD and add-on findings can be directionally wrong when the supplied
  `findings.json` was generated for a forward target instead of the rollback
  target. (API-001/API-002 findings are validated against the rollback
  target's provenance first -- see "API evidence target validation" above --
  but CRD and add-on findings are not yet.)
- CRD-001 and CRD-002 findings reflect current stored/served CRD state; they
  do not vary with `targetVersion` and are intentionally unaffected by API
  evidence target validation.
- Some potentially relevant rules are not explicitly routed through rollback
  operational readiness yet, including node skew/precondition findings,
  aggregated API availability, and CoreDNS health.
- PDB and drain findings do not become rollback failures until rollback-specific
  disruption activation evidence is available.
- Add-on rollback readiness does not yet distinguish whether the add-on itself
  must be rolled back, whether catalog operational impacts intersect the
  rollback path, or whether the installed add-on is compatible with the rollback
  target version.
- Findings cluster identity validation (see above) uses cluster name and
  region only. It does not yet use cluster ARN, AWS account ID, API-server
  endpoint, or a Kubernetes cluster UID -- none are collected today. A
  cluster recreated with the same name in the same region cannot be
  distinguished from the original by this validation.
- Findings freshness validation (see "Findings freshness validation" above)
  uses one fixed 24-hour report-wide threshold. It does not yet model that
  different evidence families (node groups, add-ons, workload health,
  disruption, CRD/webhook state) may have different volatility, and does not
  yet offer a user-configurable threshold or CLI override.
- KubePreflight does not yet re-collect live evidence at assessment time --
  freshness validation only judges the age of what was supplied via
  `--findings`; recollecting fresh live-cluster evidence remains future
  work.
- API-001/API-002 findings are not yet cluster-identity-gated (see "Why
  API-001/API-002 aren't cluster-identity-gated in this PR" above) --
  per-finding live-vs-manifest plane provenance would be needed first.
- KubePreflight does not yet recalculate API compatibility against the
  actual rollback target from live cluster evidence, or recalculate add-on
  compatibility against the rollback target version.

## Recommendation Engine

The recommendation engine is deterministic and assessment-only. It does not
execute rollback, start AWS operations, mutate Kubernetes resources, downgrade
node groups, or downgrade add-ons.

The final decision is derived from eligibility, AWS insight results,
operational readiness, and evidence completeness:

- `eligibility: unavailable` -> `do_not_proceed`
- `eligibility: unknown` -> `operator_decision_required`
- `readiness: blocked` -> `do_not_proceed`
- `readiness: insufficient_evidence` -> `operator_decision_required`
- incomplete evidence -> `operator_decision_required`
- `readiness: high_risk` -> `fix_forward_preferred`
- `readiness: ready` with complete evidence -> `rollback_preferred`

Recommendation reason codes are collected in a stable order from eligibility,
previous recommendation context, and checks. Duplicate reason codes are removed
without reordering the remaining evidence. KubePreflight only prefers rollback
when eligibility is confirmed, readiness is ready, and evidence is complete.
Incomplete or stale evidence cannot become a high-confidence rollback
recommendation.

## CLI and Reports

Rollback readiness is exposed through two read-only commands:

```bash
kubepreflight rollback plan \
  --provider eks \
  --cluster-name <cluster>

kubepreflight rollback assess \
  --provider eks \
  --cluster-name <cluster> \
  --findings findings.json
```

Both commands collect EKS eligibility and rollback-readiness insight evidence
through AWS APIs. `rollback plan` uses `pre_upgrade_posture` mode, while
`rollback assess` uses `post_upgrade_readiness` mode.

The optional `--findings` flag accepts a recent KubePreflight `findings.json`
so the assessment can include operational readiness signals such as managed
node groups, add-ons, unhealthy workloads, PDB/drain risk, API lifecycle, CRD,
webhook, and coverage evidence. If `--findings` is omitted, KubePreflight marks
that operational evidence incomplete instead of assuming it is clean.

Generated artifacts:

- `rollback-assessment.json` using
  `kubepreflight.io/rollback-assessment/v1alpha1`
- `rollback-report.md` when `--output md` or `--output all` is selected
- `rollback-report.html` when `--output html` or `--output all` is selected

The Console can display a rollback assessment from `rollback-assessment.json`
or a `?rollback=<path>` URL. The rollback Console view shows eligibility,
readiness, recommendation, confidence, evidence completeness, rollback-window
context, reason codes, and per-check evidence.

## Scope Boundary

`v0.12.0` remains read-only. Recommended operational steps may appear in reports
or action plans, but execution remains outside KubePreflight.
