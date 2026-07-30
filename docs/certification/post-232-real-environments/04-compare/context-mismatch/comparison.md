# Upgrade Readiness Comparison

- Schema version: `kubepreflight.io/scan-comparison/v1`
- **Warning:** baseline used upgradeContext "unspecified" and current used "worker_rollout" -- blocker counts and verdicts are context-aware, so review gate changes with the selected operation in mind.

| | |
|---|---|
| **Verdict** | BLOCKED |
| **Upgrade context** | unspecified → worker_rollout |
| **Readiness score** | 85 → 85 (0) |
| **New** | 0 (0 blocker(s)) |
| **Resolved** | 0 (0 blocker(s)) |
| **Not re-evaluated** | 0 |
| **Changed** | 0 |
| **Unchanged** | 1 |

## New findings (0)

None.

## Changed findings (0)

None.

## Resolved findings (0)

None.

## Not re-evaluated findings (0)

The finding was present in the baseline, but its rule was not successfully evaluated in the current report, so resolution cannot be confirmed.

None.

## Unchanged findings (1)

| Priority | Severity | Rule | Resource | Message |
|---|---|---|---|---|
| P2 | Blocker | `API-001` | default/sample | PodDisruptionBudget "default/sample" (apiVersion policy/v1beta1) in deploy.yaml uses an API version removed in Kubernetes 1.25 — this manifest will fail to apply once the cluster reaches target 1.33 |
