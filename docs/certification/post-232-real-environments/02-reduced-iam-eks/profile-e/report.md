# KubePreflight Scan Report

| | |
|---|---|
| **Cluster** | kp-cert-0729-a0fb234a |
| **Full cluster identifier** | `[redacted-arn]` |
| **Target version** | 1.34 |
| **Provider** | eks |
| **Upgrade context** | audit_only |
| **Scanned at** | 2026-07-29 17:33:49 UTC |
| **Result** | **INCOMPLETE** |
| **Summary** | 0 blocker(s), 2 warning(s), 1 operator decision(s), 0 info(s) |

> **No version upgrade required:** cluster is already running Kubernetes 1.34 (target: 1.34). Upgrade-transition checks were skipped; current-state and manifest-safety findings below were still fully evaluated.

> **Assessment incomplete:** one or more evidence sources could not be collected; absence of findings is not proof of readiness.

- AWS: describe-nodegroup:cert-ng2 [permission-denied,permissionDenied,partialDataPreserved]: operation error EKS: DescribeNodegroup, https response error StatusCode: 403, RequestID: e38e2906-6351-4d47-9de0-1a198b26172a, api error AccessDeniedException: User: [redacted-arn] is not authorized to perform: eks:DescribeNodegroup on resource: [redacted-arn] because no identity-based policy allows the eks:DescribeNodegroup action
- AWS: list-addons [permission-denied,permissionDenied,partialDataPreserved]: operation error EKS: ListAddons, https response error StatusCode: 403, RequestID: 89a036d1-0099-4551-bee6-6564f8c9d1cf, api error AccessDeniedException: User is not authorized to perform this action
- AWS: list-insights [permission-denied,permissionDenied,partialDataPreserved]: operation error EKS: ListInsights, https response error StatusCode: 403, RequestID: 84f3169b-3c5a-4a0e-9cc9-e8d434019105, api error AccessDeniedException: User: [redacted-arn] is not authorized to perform: eks:ListInsights on resource: [redacted-arn] because no identity-based policy allows the eks:ListInsights action
- AWS: list-insights-fallback [permission-denied,permissionDenied,partialDataPreserved]: operation error EKS: ListInsights, https response error StatusCode: 403, RequestID: 6773fa72-5442-4e45-8a5c-b729873182cf, api error AccessDeniedException: User: [redacted-arn] is not authorized to perform: eks:ListInsights on resource: [redacted-arn] because no identity-based policy allows the eks:ListInsights action

## Cluster Health (no version upgrade assessed)

| | |
|---|---|
| **Verdict** | INCOMPLETE |
| **Readiness score** | 90/100 |
| **Coverage** | Partial |
| **Remediation needed** | Yes |

> **Score interpretation:** The readiness score is based on findings produced by evaluated checks. Rules that were not evaluated are not penalized in the score.
>
> **Advisory:** 9 applicable rules were not fully evaluated; evidence collection was incomplete for: AWS. Review before approving the change.

| Category | Status | Blockers | Warnings | Rule IDs |
|---|---|---|---|---|
| API Compatibility | Passed | 0 | 0 |  |
| Extension APIs | Passed | 0 | 0 |  |
| Admission Webhooks | Warning | 0 | 1 | WH-005 |
| Disruption Safety | Passed | 0 | 0 |  |
| Drain Readiness | Warning | 0 | 1 | DRAIN-003 |
| Node Readiness | Passed | 0 | 0 |  |
| Add-ons | Passed | 0 | 0 |  |
| CoreDNS | Passed | 0 | 0 |  |
| Workload Health | Passed | 0 | 0 |  |
| EKS Upgrade Insights | Passed | 0 | 0 |  |

## API Compatibility

| | |
|---|---|
| **Status** | Passed |
| **Remediation needed** | No |
| **Score impact** | 0 |
| **Removed API objects** | 0 across 0 API families |
| **Deprecated API objects** | 0 across 0 API families |
| **Critical impact** | No |

## Evaluation Coverage

| | |
|---|---|
| **Evaluation coverage** | Partial |
| **Total rules** | 31 |
| **Evaluated** | 22 |
| **Not evaluated** | 0 |
| **Insufficient evidence** | 9 |
| **Failed** | 0 |
| **Not applicable** | 0 |
| **Source** | Native |

| Rule ID | Applicability | Execution state | Outcome | Reason |
|---|---|---|---|---|
| `ADDON-001` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: aws:list-addons |
| `ADDON-002` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: aws:list-addons |
| `API-001` | Applicable | Evaluated | No issue detected |  |
| `API-002` | Applicable | Evaluated | No issue detected |  |
| `APISERVICE-001` | Applicable | Evaluated | No issue detected |  |
| `COREDNS-001` | Applicable | Evaluated | No issue detected |  |
| `CRD-001` | Applicable | Evaluated | No issue detected |  |
| `CRD-002` | Applicable | Evaluated | No issue detected |  |
| `DRAIN-001` | Applicable | Evaluated | No issue detected |  |
| `DRAIN-002` | Applicable | Evaluated | No issue detected |  |
| `DRAIN-003` | Applicable | Evaluated | 1 finding(s) reported |  |
| `DRAIN-004` | Applicable | Evaluated | No issue detected |  |
| `DRAIN-005` | Applicable | Evaluated | No issue detected |  |
| `EKS-INSIGHT-001` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: aws:list-insights |
| `EKS-INSIGHT-002` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: aws:list-insights |
| `EKS-INSIGHT-003` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: aws:list-insights |
| `EKS-NG-001` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: aws:describe-nodegroup:\* |
| `EKS-NG-002` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: aws:describe-nodegroup:\* |
| `EKS-NG-003` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: aws:describe-nodegroup:\* |
| `EKS-NG-004` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: aws:describe-nodegroup:\* |
| `NET-002` | Applicable | Evaluated | No issue detected |  |
| `NODE-001` | Applicable | Evaluated | No issue detected |  |
| `NODE-002` | Applicable | Evaluated | No issue detected |  |
| `NODE-003` | Applicable | Evaluated | No issue detected |  |
| `PDB-001` | Applicable | Evaluated | No issue detected |  |
| `PDB-002` | Applicable | Evaluated | No issue detected |  |
| `WH-001` | Applicable | Evaluated | No issue detected |  |
| `WH-002` | Applicable | Evaluated | No issue detected |  |
| `WH-004` | Applicable | Evaluated | No issue detected |  |
| `WH-005` | Applicable | Evaluated | 1 finding(s) reported |  |
| `WORKLOAD-001` | Applicable | Evaluated | No issue detected |  |

## Warnings (2)

### `P3` `DRAIN-003` Deployment kube-system/coredns has a nodeSelector/required nodeAffinity satisfied by only 1 node(s) in this cluster today ([redacted-node-hostname]) — if that node is drained, no other currently-known node can host a replacement pod

Confidence: `OBSERVED` · Upgrade gate: `allow` · Impact scope: `worker_rollout, node_drain, workload_restart` · Can upgrade continue: Yes

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- qualifying node(s): [redacted-node-hostname]

**Remediation:**

```
Label additional nodes to match this workload's nodeSelector/nodeAffinity (and taint them consistently if tolerations are also required), or relax the constraint if it's broader than actually needed.
```

### `P4` `WH-005` ValidatingWebhookConfiguration "vpc-resource-validating-webhook": webhook "vnode.vpc.k8s.aws" (index 1 in .webhooks) matches nodes — this webhook covers node status updates, namespace lifecycle, or PersistentVolume operations but is fail-open (failurePolicy: Ignore), so an unavailable backend won't block those operations; confirm fail-open was an intentional choice for a webhook covering resources this sensitive

Confidence: `STATIC_CERTAIN` · Upgrade gate: `operator_decision` · Impact scope: `api_write, future_maintenance` · Can upgrade continue: No

> **Why this matters (P4):** Upgrade should not begin while workloads, nodes, or critical add-ons are unhealthy.

**Evidence:**

- webhook name: vnode.vpc.k8s.aws
- matched resource: nodes
- failurePolicy: Ignore

**Remediation:**

```
Confirm this webhook genuinely needs to validate/mutate this resource. If not, narrow its rules to exclude it.
```

## Recommended Maintenance (2)

1. **[P3/Warning] Deployment/kube-system/coredns** (DRAIN-003)

   ```
   Label additional nodes to match this workload's nodeSelector/nodeAffinity (and taint them consistently if tolerations are also required), or relax the constraint if it's broader than actually needed.
   ```

2. **[P4/Warning] ValidatingWebhookConfiguration/vpc-resource-validating-webhook** (WH-005)

   ```
   Confirm this webhook genuinely needs to validate/mutate this resource. If not, narrow its rules to exclude it.
   ```

## Evidence Appendix

Every finding's resource identity and fingerprint — cross-reference by fingerprint for waivers/dedup.

| Priority | Rule ID | Severity | Confidence | Resource | Fingerprint |
|---|---|---|---|---|---|
| P3 | DRAIN-003 | Warning | OBSERVED | Deployment/kube-system/coredns | `3ed0c0f3524e947616a24df5ec9aa5dabaebb5494c113f01222669de6132e1e4` |
| P4 | WH-005 | Warning | STATIC_CERTAIN | ValidatingWebhookConfiguration/vpc-resource-validating-webhook | `06661e5fc20e88a213fa32c9929aedcf536ca3be37984acf7c50d9e899e8d770` |
