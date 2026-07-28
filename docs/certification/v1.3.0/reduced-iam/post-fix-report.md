# KubePreflight Scan Report

| | |
|---|---|
| **Cluster** | kubepreflight-v130-cert |
| **Full cluster identifier** | `<CLUSTER_ARN>` |
| **Target version** | 1.36 |
| **Provider** | eks |
| **Upgrade context** | unspecified |
| **Scanned at** | 2026-07-27 11:18:36 UTC |
| **Result** | **INCOMPLETE** |
| **Summary** | 0 blocker(s), 3 warning(s), 1 operator decision(s), 0 info(s) |

> **No version upgrade required:** cluster is already running Kubernetes 1.36 (target: 1.36). Upgrade-transition checks were skipped; current-state and manifest-safety findings below were still fully evaluated.

> **Assessment incomplete:** one or more evidence sources could not be collected; absence of findings is not proof of readiness.

- AWS: describe-security-group:<SECURITY_GROUP_ID> [permission-denied,permissionDenied,partialDataPreserved]: operation error EC2: DescribeSecurityGroups, https response error StatusCode: 403, RequestID: 4cad4e15-27ee-4ccf-97f7-d381072fc8d3, api error UnauthorizedOperation: You are not authorized to perform this operation. User: <ASSUMED_ROLE_SESSION> is not authorized to perform: ec2:DescribeSecurityGroups because no identity-based policy allows the ec2:DescribeSecurityGroups action
- AWS: describe-security-group:<SECURITY_GROUP_ID> [permission-denied,permissionDenied,partialDataPreserved]: operation error EC2: DescribeSecurityGroups, https response error StatusCode: 403, RequestID: c9296751-a573-44fa-9418-fb1b0a176e0f, api error UnauthorizedOperation: You are not authorized to perform this operation. User: <ASSUMED_ROLE_SESSION> is not authorized to perform: ec2:DescribeSecurityGroups because no identity-based policy allows the ec2:DescribeSecurityGroups action
- AWS: describe-subnets [permission-denied,permissionDenied,partialDataPreserved]: operation error EC2: DescribeSubnets, https response error StatusCode: 403, RequestID: 8cd2182f-5c19-4268-a7e9-a2aefc53bbd7, api error UnauthorizedOperation: You are not authorized to perform this operation. User: <ASSUMED_ROLE_SESSION> is not authorized to perform: ec2:DescribeSubnets because no identity-based policy allows the ec2:DescribeSubnets action
- AWS: describe-vpc:<VPC_ID> [permission-denied,permissionDenied,partialDataPreserved]: operation error EC2: DescribeVpcs, https response error StatusCode: 403, RequestID: ac4b02de-ea01-4b60-b83b-caecb66b3fa9, api error UnauthorizedOperation: You are not authorized to perform this operation. User: <ASSUMED_ROLE_SESSION> is not authorized to perform: ec2:DescribeVpcs because no identity-based policy allows the ec2:DescribeVpcs action
- AWS: list-addons [permission-denied,permissionDenied,partialDataPreserved]: operation error EKS: ListAddons, https response error StatusCode: 403, RequestID: e6032cc6-eeb1-432a-8fc4-c1a344a98e0e, api error AccessDeniedException: User is not authorized to perform this action
- AWS: list-insights [permission-denied,permissionDenied,partialDataPreserved]: operation error EKS: ListInsights, https response error StatusCode: 403, RequestID: 4a83d069-aaa4-475d-a0e4-2e9555f5caa1, api error AccessDeniedException: User: <ASSUMED_ROLE_SESSION> is not authorized to perform: eks:ListInsights on resource: <CLUSTER_ARN> because no identity-based policy allows the eks:ListInsights action
- AWS: list-insights-fallback [permission-denied,permissionDenied,partialDataPreserved]: operation error EKS: ListInsights, https response error StatusCode: 403, RequestID: 4a570cce-86a9-4c2c-bef2-6c3f11652349, api error AccessDeniedException: User: <ASSUMED_ROLE_SESSION> is not authorized to perform: eks:ListInsights on resource: <CLUSTER_ARN> because no identity-based policy allows the eks:ListInsights action
- AWS: list-nodegroups [permission-denied,permissionDenied,partialDataPreserved]: operation error EKS: ListNodegroups, https response error StatusCode: 403, RequestID: b3406e31-314e-428a-ae95-c807cf0e3da9, api error AccessDeniedException: User: <ASSUMED_ROLE_SESSION> is not authorized to perform: eks:ListNodegroups on resource: <CLUSTER_ARN> because no identity-based policy allows the eks:ListNodegroups action

## Cluster Health (no version upgrade assessed)

| | |
|---|---|
| **Verdict** | INCOMPLETE |
| **Readiness score** | 89/100 |
| **Coverage** | Partial |
| **Remediation needed** | Yes |

> **Score interpretation:** The readiness score is based on findings produced by evaluated checks. Rules that were not evaluated are not penalized in the score.
>
> **Advisory:** evidence collection was incomplete for: AWS. Review before approving the change.

| Category | Status | Blockers | Warnings | Rule IDs |
|---|---|---|---|---|
| API Compatibility | Passed | 0 | 0 |  |
| Extension APIs | Passed | 0 | 0 |  |
| Admission Webhooks | Warning | 0 | 1 | WH-005 |
| Disruption Safety | Passed | 0 | 0 |  |
| Drain Readiness | Warning | 0 | 2 | DRAIN-003 |
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
| **Evaluation coverage** | Complete |
| **Total rules** | 31 |
| **Evaluated** | 31 |
| **Not evaluated** | 0 |
| **Insufficient evidence** | 0 |
| **Failed** | 0 |
| **Not applicable** | 0 |
| **Source** | Native |

| Rule ID | Applicability | Execution state | Outcome | Reason |
|---|---|---|---|---|
| `ADDON-001` | Applicable | Evaluated | No issue detected |  |
| `ADDON-002` | Applicable | Evaluated | No issue detected |  |
| `API-001` | Applicable | Evaluated | No issue detected |  |
| `API-002` | Applicable | Evaluated | No issue detected |  |
| `APISERVICE-001` | Applicable | Evaluated | No issue detected |  |
| `COREDNS-001` | Applicable | Evaluated | No issue detected |  |
| `CRD-001` | Applicable | Evaluated | No issue detected |  |
| `CRD-002` | Applicable | Evaluated | No issue detected |  |
| `DRAIN-001` | Applicable | Evaluated | No issue detected |  |
| `DRAIN-002` | Applicable | Evaluated | No issue detected |  |
| `DRAIN-003` | Applicable | Evaluated | 2 finding(s) reported |  |
| `DRAIN-004` | Applicable | Evaluated | No issue detected |  |
| `DRAIN-005` | Applicable | Evaluated | No issue detected |  |
| `EKS-INSIGHT-001` | Applicable | Evaluated | No issue detected |  |
| `EKS-INSIGHT-002` | Applicable | Evaluated | No issue detected |  |
| `EKS-INSIGHT-003` | Applicable | Evaluated | No issue detected |  |
| `EKS-NG-001` | Applicable | Evaluated | No issue detected |  |
| `EKS-NG-002` | Applicable | Evaluated | No issue detected |  |
| `EKS-NG-003` | Applicable | Evaluated | No issue detected |  |
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

### `P3` `DRAIN-003` Deployment kube-system/coredns has a nodeSelector/required nodeAffinity satisfied by only 1 node(s) in this cluster today (<NODE_PRIVATE_HOSTNAME>) — if that node is drained, no other currently-known node can host a replacement pod

Confidence: `OBSERVED` · Upgrade gate: `allow` · Impact scope: `worker_rollout, node_drain, workload_restart` · Can upgrade continue: Yes

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- qualifying node(s): <NODE_PRIVATE_HOSTNAME>

**Remediation:**

```
Label additional nodes to match this workload's nodeSelector/nodeAffinity (and taint them consistently if tolerations are also required), or relax the constraint if it's broader than actually needed.
```

### `P3` `DRAIN-003` Deployment kube-system/metrics-server has a nodeSelector/required nodeAffinity satisfied by only 1 node(s) in this cluster today (<NODE_PRIVATE_HOSTNAME>) — if that node is drained, no other currently-known node can host a replacement pod

Confidence: `OBSERVED` · Upgrade gate: `allow` · Impact scope: `worker_rollout, node_drain, workload_restart` · Can upgrade continue: Yes

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- qualifying node(s): <NODE_PRIVATE_HOSTNAME>

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

## Recommended Maintenance (3)

1. **[P3/Warning] Deployment/kube-system/coredns** (DRAIN-003)

   ```
   Label additional nodes to match this workload's nodeSelector/nodeAffinity (and taint them consistently if tolerations are also required), or relax the constraint if it's broader than actually needed.
   ```

2. **[P3/Warning] Deployment/kube-system/metrics-server** (DRAIN-003)

   ```
   Label additional nodes to match this workload's nodeSelector/nodeAffinity (and taint them consistently if tolerations are also required), or relax the constraint if it's broader than actually needed.
   ```

3. **[P4/Warning] ValidatingWebhookConfiguration/vpc-resource-validating-webhook** (WH-005)

   ```
   Confirm this webhook genuinely needs to validate/mutate this resource. If not, narrow its rules to exclude it.
   ```

## Evidence Appendix

Every finding's resource identity and fingerprint — cross-reference by fingerprint for waivers/dedup.

| Priority | Rule ID | Severity | Confidence | Resource | Fingerprint |
|---|---|---|---|---|---|
| P3 | DRAIN-003 | Warning | OBSERVED | Deployment/kube-system/coredns | `d5c1e9da1b99cd6c8ece2807daa4285ca2b2751e05c3f9ba3a12a7ad498ff7e5` |
| P3 | DRAIN-003 | Warning | OBSERVED | Deployment/kube-system/metrics-server | `8110f1ce72a44a1d9d399ac5c300fae82318ec4fba1f88595469de3ad1b1421d` |
| P4 | WH-005 | Warning | STATIC_CERTAIN | ValidatingWebhookConfiguration/vpc-resource-validating-webhook | `a6f358136d35ceaa49be081be84d2ee49cd06dd788f3ed04d2e95530f958629a` |
