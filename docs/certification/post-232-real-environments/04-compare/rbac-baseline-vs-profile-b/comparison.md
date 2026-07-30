# Upgrade Readiness Comparison

- Schema version: `kubepreflight.io/scan-comparison/v1`

| | |
|---|---|
| **Verdict** | PASSED_WITH_WARNINGS → INCOMPLETE |
| **Upgrade context** | audit_only → audit_only |
| **Readiness score** | 88 → 88 (0) |
| **New** | 0 (0 blocker(s)) |
| **Resolved** | 0 (0 blocker(s)) |
| **Not re-evaluated** | 0 |
| **Changed** | 0 |
| **Unchanged** | 4 |

## New findings (0)

None.

## Changed findings (0)

None.

## Resolved findings (0)

None.

## Not re-evaluated findings (0)

The finding was present in the baseline, but its rule was not successfully evaluated in the current report, so resolution cannot be confirmed.

None.

## Unchanged findings (4)

| Priority | Severity | Rule | Resource | Message |
|---|---|---|---|---|
| P3 | Warning | `DRAIN-001` | local-path-storage/local-path-provisioner | Deployment local-path-storage/local-path-provisioner runs a single replica (desired: 1, ready: 1, available: 1) — when its pod is evicted (node drain, node upgrade, or any voluntary disruption), this workload has zero available replicas until a replacement schedules and becomes Ready elsewhere; no PodDisruptionBudget protects this workload |
| P3 | Warning | `DRAIN-003` | kube-system/coredns | Deployment kube-system/coredns has a nodeSelector/required nodeAffinity satisfied by only 1 node(s) in this cluster today (kp-cert-rbac-control-plane) — if that node is drained, no other currently-known node can host a replacement pod |
| P3 | Warning | `DRAIN-003` | local-path-storage/local-path-provisioner | Deployment local-path-storage/local-path-provisioner has a nodeSelector/required nodeAffinity satisfied by only 1 node(s) in this cluster today (kp-cert-rbac-control-plane) — if that node is drained, no other currently-known node can host a replacement pod |
| P3 | Warning | `NODE-003` | local-path-storage/local-path-provisioner | Deprecated control-plane node label dependency: Deployment local-path-storage/local-path-provisioner depends on node-role.kubernetes.io/master. This does not necessarily block an in-place Kubernetes upgrade while existing nodes retain the label. The workload may fail to schedule after a control-plane node replacement, label removal, or pod restart if no node exposes the legacy label. |
