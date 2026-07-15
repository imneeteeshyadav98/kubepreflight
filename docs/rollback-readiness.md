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

## Scope Boundary

`v0.12.0` remains read-only. Recommended operational steps may appear in reports
or action plans, but execution remains outside KubePreflight.
