# KubePreflight Scan Report

| | |
|---|---|
| **Cluster** | kp-cert-0729-a0fb234a |
| **Full cluster identifier** | `[redacted-arn]` |
| **Target version** | 1.34 |
| **Provider** | eks |
| **Upgrade context** | audit_only |
| **Scanned at** | 2026-07-29 17:34:11 UTC |
| **Result** | **INCOMPLETE** |
| **Summary** | 0 blocker(s), 3 warning(s), 1 operator decision(s), 1 info(s) |

> **No version upgrade required:** cluster is already running Kubernetes 1.34 (target: 1.34). Upgrade-transition checks were skipped; current-state and manifest-safety findings below were still fully evaluated.

> **Assessment incomplete:** one or more evidence sources could not be collected; absence of findings is not proof of readiness.

- AWS: list-addons [permission-denied,permissionDenied,partialDataPreserved]: operation error EKS: ListAddons, https response error StatusCode: 403, RequestID: a82d0bd9-92e8-460b-9af6-b583cbdcfa56, api error AccessDeniedException: User is not authorized to perform this action
- AWS: list-insights [permission-denied,permissionDenied,partialDataPreserved]: operation error EKS: ListInsights, https response error StatusCode: 403, RequestID: 1dd40643-39e1-4b66-bb04-a8b9c68f3a94, api error AccessDeniedException: User: [redacted-arn] is not authorized to perform: eks:ListInsights on resource: [redacted-arn] because no identity-based policy allows the eks:ListInsights action
- AWS: list-insights-fallback [permission-denied,permissionDenied,partialDataPreserved]: operation error EKS: ListInsights, https response error StatusCode: 403, RequestID: 4cdc77c0-1579-4988-bb52-9af550603b87, api error AccessDeniedException: User: [redacted-arn] is not authorized to perform: eks:ListInsights on resource: [redacted-arn] because no identity-based policy allows the eks:ListInsights action

## Cluster Health (no version upgrade assessed)

| | |
|---|---|
| **Verdict** | INCOMPLETE |
| **Readiness score** | 85/100 |
| **Coverage** | Partial |
| **Remediation needed** | Yes |

> **Score interpretation:** The readiness score is based on findings produced by evaluated checks. Rules that were not evaluated are not penalized in the score.
>
> **Advisory:** 5 applicable rules were not fully evaluated; evidence collection was incomplete for: AWS. Review before approving the change.

| Category | Status | Blockers | Warnings | Rule IDs |
|---|---|---|---|---|
| API Compatibility | Passed | 0 | 0 |  |
| Extension APIs | Passed | 0 | 0 |  |
| Admission Webhooks | Warning | 0 | 1 | WH-005 |
| Disruption Safety | Passed | 0 | 0 |  |
| Drain Readiness | Warning | 0 | 1 | DRAIN-003 |
| Node Readiness | Warning | 0 | 1 | EKS-NG-002, EKS-NG-003 |
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
| **Evaluated** | 26 |
| **Not evaluated** | 0 |
| **Insufficient evidence** | 5 |
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
| `EKS-NG-001` | Applicable | Evaluated | No issue detected |  |
| `EKS-NG-002` | Applicable | Evaluated | 1 finding(s) reported |  |
| `EKS-NG-003` | Applicable | Evaluated | 1 finding(s) reported |  |
| `EKS-NG-004` | Applicable | Evaluated | No issue detected |  |
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

## Warnings (3)

### `P3` `DRAIN-003` Deployment kube-system/coredns has a nodeSelector/required nodeAffinity satisfied by only 1 node(s) in this cluster today ([redacted-node-hostname]) — if that node is drained, no other currently-known node can host a replacement pod

Confidence: `OBSERVED` · Upgrade gate: `allow` · Impact scope: `worker_rollout, node_drain, workload_restart` · Can upgrade continue: Yes

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- qualifying node(s): [redacted-node-hostname]

**Remediation:**

```
Label additional nodes to match this workload's nodeSelector/nodeAffinity (and taint them consistently if tolerations are also required), or relax the constraint if it's broader than actually needed.
```

### `P3` `EKS-NG-002` Managed node group "cert-ng2" desired size equals or is below minimum size. Rolling update may have limited disruption headroom.

Confidence: `PROVIDER_REPORTED` · Upgrade gate: `allow` · Impact scope: `-` · Can upgrade continue: Yes

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- desiredSize: 1
- minSize: 1
- maxSize: 1

**Remediation:**

```
Review node group capacity and disruption budgets before upgrade. Consider temporarily increasing desired capacity or otherwise creating eviction headroom for the change window.
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

### `P4` `EKS-NG-003` Managed node group "cert-ng2" uses a launch template/custom AMI. Validate AMI, bootstrap, kubelet, and launch template upgrade path manually.

Confidence: `PROVIDER_REPORTED` · Upgrade gate: `allow` · Impact scope: `-` · Can upgrade continue: Yes

> **Why this matters (P4):** Upgrade should not begin while workloads, nodes, or critical add-ons are unhealthy.

**Evidence:**

- launchTemplate: true
- amiType: AL2023_x86_64_STANDARD

**Remediation:**

```
Manually validate the launch template or custom AMI upgrade path, including bootstrap configuration, kubelet version, user data, and AMI release process.
```

## Recommended Maintenance (3)

1. **[P3/Warning] Deployment/kube-system/coredns** (DRAIN-003)

   ```
   Label additional nodes to match this workload's nodeSelector/nodeAffinity (and taint them consistently if tolerations are also required), or relax the constraint if it's broader than actually needed.
   ```

2. **[P3/Warning] EKSNodegroup/cert-ng2** (EKS-NG-002)

   ```
   Review node group capacity and disruption budgets before upgrade. Consider temporarily increasing desired capacity or otherwise creating eviction headroom for the change window.
   ```

3. **[P4/Warning] ValidatingWebhookConfiguration/vpc-resource-validating-webhook** (WH-005)

   ```
   Confirm this webhook genuinely needs to validate/mutate this resource. If not, narrow its rules to exclude it.
   ```

## Evidence Appendix

Every finding's resource identity and fingerprint — cross-reference by fingerprint for waivers/dedup.

| Priority | Rule ID | Severity | Confidence | Resource | Fingerprint |
|---|---|---|---|---|---|
| P3 | DRAIN-003 | Warning | OBSERVED | Deployment/kube-system/coredns | `3ed0c0f3524e947616a24df5ec9aa5dabaebb5494c113f01222669de6132e1e4` |
| P3 | EKS-NG-002 | Warning | PROVIDER_REPORTED | EKSNodegroup/cert-ng2 | `81905013d9bae85ddd70e590d9efd63814c1333a1862c1c3ea6df3f074c0d23e` |
| P4 | WH-005 | Warning | STATIC_CERTAIN | ValidatingWebhookConfiguration/vpc-resource-validating-webhook | `06661e5fc20e88a213fa32c9929aedcf536ca3be37984acf7c50d9e899e8d770` |
| P4 | EKS-NG-003 | Info | PROVIDER_REPORTED | EKSNodegroup/cert-ng2 | `972a9d23d2e8aa7308352fab4363afd48c369b6f8bd487290e43f7222048dd18` |
