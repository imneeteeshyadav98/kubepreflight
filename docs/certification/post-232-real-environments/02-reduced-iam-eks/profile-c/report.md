# KubePreflight Scan Report

| | |
|---|---|
| **Cluster** | kp-cert-0729-a0fb234a |
| **Full cluster identifier** | `[redacted-arn]` |
| **Target version** | 1.34 |
| **Provider** | eks |
| **Upgrade context** | audit_only |
| **Scanned at** | 2026-07-29 17:33:06 UTC |
| **Result** | **INCOMPLETE** |
| **Summary** | 0 blocker(s), 2 warning(s), 1 operator decision(s), 1 info(s) |

> **No version upgrade required:** cluster is already running Kubernetes 1.34 (target: 1.34). Upgrade-transition checks were skipped; current-state and manifest-safety findings below were still fully evaluated.

> **Assessment incomplete:** one or more evidence sources could not be collected; absence of findings is not proof of readiness.

- AWS: describe-insight:402201dc-5743-471f-8647-e20826793694 [permission-denied,permissionDenied,partialDataPreserved]: operation error EKS: DescribeInsight, https response error StatusCode: 403, RequestID: 52de3567-4337-4e7f-b960-df5bd9381b33, api error AccessDeniedException: User: [redacted-arn] is not authorized to perform: eks:DescribeInsight on resource: [redacted-arn] because no identity-based policy allows the eks:DescribeInsight action
- AWS: describe-insight:81c25cde-e866-479b-bd24-eeb4fc46d867 [permission-denied,permissionDenied,partialDataPreserved]: operation error EKS: DescribeInsight, https response error StatusCode: 403, RequestID: 1c08bd98-0272-4b89-991c-ad4e3716ab43, api error AccessDeniedException: User: [redacted-arn] is not authorized to perform: eks:DescribeInsight on resource: [redacted-arn] because no identity-based policy allows the eks:DescribeInsight action
- AWS: list-addons [permission-denied,permissionDenied,partialDataPreserved]: operation error EKS: ListAddons, https response error StatusCode: 403, RequestID: 7b40e5ca-1a9c-4943-8d41-f457351f44fe, api error AccessDeniedException: User is not authorized to perform this action
- AWS: list-nodegroups [permission-denied,permissionDenied,partialDataPreserved]: operation error EKS: ListNodegroups, https response error StatusCode: 403, RequestID: 9a3e2714-ef94-4572-8cee-03b08752bf1e, api error AccessDeniedException: User: [redacted-arn] is not authorized to perform: eks:ListNodegroups on resource: [redacted-arn] because no identity-based policy allows the eks:ListNodegroups action

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
| EKS Upgrade Insights | Passed | 0 | 0 | EKS-INSIGHT-003 |

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
| `EKS-INSIGHT-001` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: aws:describe-insight:\* |
| `EKS-INSIGHT-002` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: aws:describe-insight:\* |
| `EKS-INSIGHT-003` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: aws:describe-insight:\* |
| `EKS-NG-001` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: aws:list-nodegroups |
| `EKS-NG-002` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: aws:list-nodegroups |
| `EKS-NG-003` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: aws:list-nodegroups |
| `EKS-NG-004` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: aws:list-nodegroups |
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

## Info (1)

### `P4` `EKS-INSIGHT-003` EKS upgrade insight "EKS add-on version compatibility" reports UNKNOWN for Kubernetes 1.35. Treat this as AWS-native context and verify with a fresh scan before upgrade.

Confidence: `PROVIDER_REPORTED` · Upgrade gate: `allow` · Impact scope: `-` · Can upgrade continue: Yes

> **Why this matters (P4):** Upgrade should not begin while workloads, nodes, or critical add-ons are unhealthy.

**Evidence:**

- insight id: 43a5a377-b5cd-45f7-a32f-f6d5f2d20dac
- status: UNKNOWN
- kubernetes version: 1.35
- reason: Unable to determine version compatibility of EKS add-ons.
- last refreshed: 2026-07-29T10:19:05Z
- last transition: 2026-07-29T10:19:05Z
- recommendation: Upgrade your EKS add-on to a newer version compatible with the next Kubernetes version.
- add-on compatibility detail: kube-proxy compatible versions: v1.34.6-eksbuild.17, v1.35.0-eksbuild.2, v1.35.2-eksbuild.4, v1.35.3-eksbuild.2, v1.35.3-eksbuild.5, v1.35.3-eksbuild.8, v1.35.3-eksbuild.11, v1.35.3-eksbuild.13, v1.35.3-eksbuild.17
- add-on compatibility detail: vpc-cni compatible versions: v1.22.3-eksbuild.1, v1.22.4-eksbuild.3
- freshness note: AWS-native upgrade readiness checks from Amazon EKS. Insights may be up to 24 hours old; re-check after remediation.

**Remediation:**

```
Upgrade your EKS add-on to a newer version compatible with the next Kubernetes version.

AWS-native upgrade readiness checks from Amazon EKS. Insights may be up to 24 hours old; re-check after remediation.
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
| P4 | EKS-INSIGHT-003 | Info | PROVIDER_REPORTED | EKSUpgradeInsight/EKS add-on version compatibility | `8e03288c680c109b2b5e8fa7847fc7feaf918d0dbcad1ac498e4dbfa252841f5` |
