# KubePreflight Scan Report

| | |
|---|---|
| **Cluster** | kp-cert-documented |
| **Target version** | 1.33 |
| **Provider** | cluster-only |
| **Upgrade context** | audit_only |
| **Scanned at** | 2026-07-29 08:54:17 UTC |
| **Result** | **INCOMPLETE** |
| **Summary** | 0 blocker(s), 4 warning(s), 0 operator decision(s), 0 info(s) |

> **Assessment incomplete:** one or more evidence sources could not be collected; absence of findings is not proof of readiness.

- Kubernetes: deprecated-api:extensions/v1beta1, Resource=podsecuritypolicies [permission-denied,permissionDenied,partialDataPreserved]: podsecuritypolicies.extensions is forbidden: User "system:serviceaccount:default:kp-cert-documented" cannot list resource "podsecuritypolicies" in API group "extensions" at the cluster scope
- Kubernetes: deprecated-api:storage.k8s.io/v1beta1, Resource=csistoragecapacities [permission-denied,permissionDenied,partialDataPreserved]: csistoragecapacities.storage.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-documented" cannot list resource "csistoragecapacities" in API group "storage.k8s.io" at the cluster scope
- Kubernetes: persistentvolumeclaims [permission-denied,permissionDenied,partialDataPreserved]: persistentvolumeclaims is forbidden: User "system:serviceaccount:default:kp-cert-documented" cannot list resource "persistentvolumeclaims" in API group "" at the cluster scope
- Kubernetes: persistentvolumes [permission-denied,permissionDenied,partialDataPreserved]: persistentvolumes is forbidden: User "system:serviceaccount:default:kp-cert-documented" cannot list resource "persistentvolumes" in API group "" at the cluster scope

## Upgrade Readiness

| | |
|---|---|
| **Verdict** | INCOMPLETE |
| **Readiness score** | 88/100 |
| **Coverage** | Partial |
| **Upgrade continue** | No |

> **Score interpretation:** The readiness score is based on findings produced by evaluated checks. Rules that were not evaluated are not penalized in the score.
>
> **Advisory:** 3 applicable rules were not fully evaluated; evidence collection was incomplete for: Kubernetes. Review before approving the change.

| Category | Status | Blockers | Warnings | Rule IDs |
|---|---|---|---|---|
| API Compatibility | Passed | 0 | 0 |  |
| Extension APIs | Passed | 0 | 0 |  |
| Admission Webhooks | Passed | 0 | 0 |  |
| Disruption Safety | Passed | 0 | 0 |  |
| Drain Readiness | Warning | 0 | 3 | DRAIN-001, DRAIN-003 |
| Node Readiness | Warning | 0 | 1 | NODE-003 |
| Add-ons | Passed | 0 | 0 |  |
| CoreDNS | Passed | 0 | 0 |  |
| Workload Health | Passed | 0 | 0 |  |
| EKS Upgrade Insights | Passed | 0 | 0 |  |

## API Compatibility

| | |
|---|---|
| **Status** | Passed |
| **Upgrade continue** | Yes |
| **Score impact** | 0 |
| **Removed API objects** | 0 across 0 API families |
| **Deprecated API objects** | 0 across 0 API families |
| **Critical impact** | No |

## Evaluation Coverage

| | |
|---|---|
| **Evaluation coverage** | Partial |
| **Total rules** | 31 |
| **Evaluated** | 19 |
| **Not evaluated** | 0 |
| **Insufficient evidence** | 3 |
| **Failed** | 0 |
| **Not applicable** | 9 |
| **Source** | Native |

| Rule ID | Applicability | Execution state | Outcome | Reason |
|---|---|---|---|---|
| `ADDON-001` | Applicable | Evaluated | No issue detected |  |
| `ADDON-002` | Applicable | Evaluated | No issue detected |  |
| `API-001` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:deprecated-api:\* |
| `API-002` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:deprecated-api:\* |
| `APISERVICE-001` | Applicable | Evaluated | No issue detected |  |
| `COREDNS-001` | Applicable | Evaluated | No issue detected |  |
| `CRD-001` | Applicable | Evaluated | No issue detected |  |
| `CRD-002` | Applicable | Evaluated | No issue detected |  |
| `DRAIN-001` | Applicable | Evaluated | 1 finding(s) reported |  |
| `DRAIN-002` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:persistentvolumeclaims, kubernetes:persistentvolumes |
| `DRAIN-003` | Applicable | Evaluated | 2 finding(s) reported |  |
| `DRAIN-004` | Applicable | Evaluated | No issue detected |  |
| `DRAIN-005` | Applicable | Evaluated | No issue detected |  |
| `EKS-INSIGHT-001` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `EKS-INSIGHT-002` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `EKS-INSIGHT-003` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `EKS-NG-001` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `EKS-NG-002` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `EKS-NG-003` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `EKS-NG-004` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `NET-002` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `NODE-001` | Applicable | Evaluated | No issue detected |  |
| `NODE-002` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `NODE-003` | Applicable | Evaluated | 1 finding(s) reported |  |
| `PDB-001` | Applicable | Evaluated | No issue detected |  |
| `PDB-002` | Applicable | Evaluated | No issue detected |  |
| `WH-001` | Applicable | Evaluated | No issue detected |  |
| `WH-002` | Applicable | Evaluated | No issue detected |  |
| `WH-004` | Applicable | Evaluated | No issue detected |  |
| `WH-005` | Applicable | Evaluated | No issue detected |  |
| `WORKLOAD-001` | Applicable | Evaluated | No issue detected |  |

## Warnings (4)

### `P3` `DRAIN-001` Deployment local-path-storage/local-path-provisioner runs a single replica (desired: 1, ready: 1, available: 1) — when its pod is evicted (node drain, node upgrade, or any voluntary disruption), this workload has zero available replicas until a replacement schedules and becomes Ready elsewhere; no PodDisruptionBudget protects this workload

Confidence: `STATIC_CERTAIN` · Upgrade gate: `allow` · Impact scope: `worker_rollout, node_drain, workload_restart, current_health` · Can upgrade continue: Yes

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- desired replicas: 1
- ready replicas: 1
- available replicas: 1
- rollout strategy: RollingUpdate
- PodDisruptionBudget(s): none
- affected pod(s): local-path-provisioner-7dc846544d-mgtqc

**Remediation:**

```
Increase replicas to create real eviction headroom (a healthy PDB alone doesn't prevent the capacity gap), or explicitly accept single-replica downtime for this workload and document it. If this workload can't run multiple replicas (e.g. a singleton controller with leader election), consider a PodDisruptionBudget with minAvailable: 0 combined with a documented manual coordination process for drains.
```

### `P3` `NODE-003` Deprecated control-plane node label dependency: Deployment local-path-storage/local-path-provisioner depends on node-role.kubernetes.io/master. This does not necessarily block an in-place Kubernetes upgrade while existing nodes retain the label. The workload may fail to schedule after a control-plane node replacement, label removal, or pod restart if no node exposes the legacy label.

Confidence: `STATIC_CERTAIN` · Upgrade gate: `allow` · Impact scope: `worker_rollout, node_drain, workload_restart, future_maintenance` · Can upgrade continue: Yes

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- references node-role.kubernetes.io/master at spec.template.spec.tolerations[1].key
- replacement label: node-role.kubernetes.io/control-plane (kubeadm stopped adding the master label to new control-plane nodes in Kubernetes 1.24)

**Remediation:**

```
Replace deprecated node-role.kubernetes.io/master references with node-role.kubernetes.io/control-plane, or migrate to an explicit stable node label managed by the platform team. Validate that all target nodes already carry the replacement label before changing selectors or affinities — changing the selector first strands the workload with no schedulable nodes.
```

### `P3` `DRAIN-003` Deployment kube-system/coredns has a nodeSelector/required nodeAffinity satisfied by only 1 node(s) in this cluster today (kp-cert-rbac-control-plane) — if that node is drained, no other currently-known node can host a replacement pod

Confidence: `OBSERVED` · Upgrade gate: `allow` · Impact scope: `worker_rollout, node_drain, workload_restart` · Can upgrade continue: Yes

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- qualifying node(s): kp-cert-rbac-control-plane

**Remediation:**

```
Label additional nodes to match this workload's nodeSelector/nodeAffinity (and taint them consistently if tolerations are also required), or relax the constraint if it's broader than actually needed.
```

### `P3` `DRAIN-003` Deployment local-path-storage/local-path-provisioner has a nodeSelector/required nodeAffinity satisfied by only 1 node(s) in this cluster today (kp-cert-rbac-control-plane) — if that node is drained, no other currently-known node can host a replacement pod

Confidence: `OBSERVED` · Upgrade gate: `allow` · Impact scope: `worker_rollout, node_drain, workload_restart` · Can upgrade continue: Yes

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- qualifying node(s): kp-cert-rbac-control-plane

**Remediation:**

```
Label additional nodes to match this workload's nodeSelector/nodeAffinity (and taint them consistently if tolerations are also required), or relax the constraint if it's broader than actually needed.
```

## Next Actions (2)

1. **[P3/Warning] Deployment/local-path-storage/local-path-provisioner** (DRAIN-001, DRAIN-003, NODE-003)

   ```
   Increase replicas to create real eviction headroom (a healthy PDB alone doesn't prevent the capacity gap), or explicitly accept single-replica downtime for this workload and document it. If this workload can't run multiple replicas (e.g. a singleton controller with leader election), consider a PodDisruptionBudget with minAvailable: 0 combined with a documented manual coordination process for drains.
   ```

   Also see `NODE-003`: Replace deprecated node-role.kubernetes.io/master references with node-role.kubernetes.io/control-plane, or migrate to an explicit stable node label managed by the platform team. Validate that all target nodes already carry the replacement label before changing selectors or affinities — changing the selector first strands the workload with no schedulable nodes.

   Also see `DRAIN-003`: Label additional nodes to match this workload's nodeSelector/nodeAffinity (and taint them consistently if tolerations are also required), or relax the constraint if it's broader than actually needed.

2. **[P3/Warning] Deployment/kube-system/coredns** (DRAIN-003)

   ```
   Label additional nodes to match this workload's nodeSelector/nodeAffinity (and taint them consistently if tolerations are also required), or relax the constraint if it's broader than actually needed.
   ```

## Evidence Appendix

Every finding's resource identity and fingerprint — cross-reference by fingerprint for waivers/dedup.

| Priority | Rule ID | Severity | Confidence | Resource | Fingerprint |
|---|---|---|---|---|---|
| P3 | DRAIN-001 | Warning | STATIC_CERTAIN | Deployment/local-path-storage/local-path-provisioner | `a60e526af552aef8a2041d4be2395833924b281f3e430464b69fe873f25b62d0` |
| P3 | NODE-003 | Warning | STATIC_CERTAIN | Deployment/local-path-storage/local-path-provisioner | `2028b2647ae0229156237794304bd4812e24461da10e2882ca83a38d28ad0f2c` |
| P3 | DRAIN-003 | Warning | OBSERVED | Deployment/kube-system/coredns | `73c521dab2047ab1cd01c673e0921ac924505ad09cc4b943f0c8504389820935` |
| P3 | DRAIN-003 | Warning | OBSERVED | Deployment/local-path-storage/local-path-provisioner | `3f96544063f8cc40e09fd5841e66d7e87afa94c8ea290a8039fecffc7d905c57` |
