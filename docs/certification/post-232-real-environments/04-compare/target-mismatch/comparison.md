# Upgrade Readiness Comparison

- Schema version: `kubepreflight.io/scan-comparison/v1`
- **Warning:** baseline was scanned at target-version "1.33" and current at "1.36" -- fingerprints are scoped to target version, so genuinely unchanged findings will show up as a new+resolved pair instead of unchanged. Re-scan both at the same target version for an accurate diff.

| | |
|---|---|
| **Verdict** | BLOCKED |
| **Upgrade context** | unspecified → unspecified |
| **Readiness score** | 85 → 85 (0) |
| **New** | 1 (1 blocker(s)) |
| **Resolved** | 1 (1 blocker(s)) |
| **Not re-evaluated** | 0 |
| **Changed** | 0 |
| **Unchanged** | 0 |

## New findings (1)

| Priority | Severity | Rule | Resource | Message |
|---|---|---|---|---|
| P2 | Blocker | `API-001` | default/sample | PodDisruptionBudget "default/sample" (apiVersion policy/v1beta1) in deploy.yaml uses an API version removed in Kubernetes 1.25 — this manifest will fail to apply once the cluster reaches target 1.36 |

## Changed findings (0)

None.

## Resolved findings (1)

| Priority | Severity | Rule | Resource | Message |
|---|---|---|---|---|
| P2 | Blocker | `API-001` | default/sample | PodDisruptionBudget "default/sample" (apiVersion policy/v1beta1) in deploy.yaml uses an API version removed in Kubernetes 1.25 — this manifest will fail to apply once the cluster reaches target 1.33 |

## Not re-evaluated findings (0)

The finding was present in the baseline, but its rule was not successfully evaluated in the current report, so resolution cannot be confirmed.

None.

## Unchanged findings (0)

None.
