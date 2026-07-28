# KubePreflight Scan Report

| | |
|---|---|
| **Cluster** | kubepreflight-v130-cert |
| **Full cluster identifier** | `<CLUSTER_ARN>` |
| **Target version** | 1.36 |
| **Provider** | eks |
| **Upgrade context** | unspecified |
| **Scanned at** | 2026-07-27 09:50:07 UTC |
| **Result** | **PASSED_WITH_WARNINGS** |
| **Summary** | 0 blocker(s), 7 warning(s), 1 operator decision(s), 1 info(s) |

> **No version upgrade required:** cluster is already running Kubernetes 1.36 (target: 1.36). Upgrade-transition checks were skipped; current-state and manifest-safety findings below were still fully evaluated.

## Cluster Health (no version upgrade assessed)

| | |
|---|---|
| **Verdict** | PASSED_WITH_WARNINGS |
| **Readiness score** | 77/100 |
| **Remediation needed** | Yes |

| Category | Status | Blockers | Warnings | Rule IDs |
|---|---|---|---|---|
| API Compatibility | Passed | 0 | 0 |  |
| Extension APIs | Passed | 0 | 0 |  |
| Admission Webhooks | Warning | 0 | 1 | WH-005 |
| Disruption Safety | Passed | 0 | 0 |  |
| Drain Readiness | Warning | 0 | 2 | DRAIN-003 |
| Node Readiness | Warning | 0 | 1 | EKS-NG-002, EKS-NG-003 |
| Add-ons | Warning | 0 | 3 | ADDON-002 |
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
| `ADDON-002` | Applicable | Evaluated | 3 finding(s) reported |  |
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

## Warnings (7)

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

### `P3` `ADDON-002` EKS add-on "coredns" version v1.14.2-eksbuild.4 has no compatibility catalog entry for target Kubernetes 1.36 — confirm compatibility before starting the upgrade

Confidence: `PROVIDER_REPORTED` · Upgrade gate: `allow` · Impact scope: `-` · Can upgrade continue: Yes

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- installed add-on: coredns
- current version: v1.14.2-eksbuild.4
- target Kubernetes version: 1.36
- minimum compatible version: unknown
- recommended upgrade version: unknown
- compatibility status: unknown
- catalog source: no catalog entry for provider=eks add-on target
- required upgrade order: 3. CoreDNS after VPC CNI and kube-proxy, before storage CSI add-ons

**Remediation:**

```
Verify this EKS managed add-on against provider compatibility metadata before upgrading. Missing catalog coverage is a warning, not proof of incompatibility.
```

### `P3` `ADDON-002` EKS add-on "kube-proxy" version v1.36.0-eksbuild.7 has no compatibility catalog entry for target Kubernetes 1.36 — confirm compatibility before starting the upgrade

Confidence: `PROVIDER_REPORTED` · Upgrade gate: `allow` · Impact scope: `-` · Can upgrade continue: Yes

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- installed add-on: kube-proxy
- current version: v1.36.0-eksbuild.7
- target Kubernetes version: 1.36
- minimum compatible version: unknown
- recommended upgrade version: unknown
- compatibility status: unknown
- catalog source: no catalog entry for provider=eks add-on target
- required upgrade order: 2. kube-proxy after VPC CNI and before CoreDNS/storage add-ons

**Remediation:**

```
Verify this EKS managed add-on against provider compatibility metadata before upgrading. Missing catalog coverage is a warning, not proof of incompatibility.
```

### `P3` `ADDON-002` EKS add-on "vpc-cni" version v1.21.2-eksbuild.2 has no compatibility catalog entry for target Kubernetes 1.36 — confirm compatibility before starting the upgrade

Confidence: `PROVIDER_REPORTED` · Upgrade gate: `allow` · Impact scope: `-` · Can upgrade continue: Yes

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- installed add-on: vpc-cni
- current version: v1.21.2-eksbuild.2
- target Kubernetes version: 1.36
- minimum compatible version: unknown
- recommended upgrade version: unknown
- compatibility status: unknown
- catalog source: no catalog entry for provider=eks add-on target
- required upgrade order: 1. Amazon VPC CNI before kube-proxy and DNS/storage add-ons

**Remediation:**

```
Verify this EKS managed add-on against provider compatibility metadata before upgrading. Missing catalog coverage is a warning, not proof of incompatibility.
```

### `P3` `EKS-NG-002` Managed node group "certification-ng" desired size equals or is below minimum size. Rolling update may have limited disruption headroom.

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

### `P4` `EKS-NG-003` Managed node group "certification-ng" uses a launch template/custom AMI. Validate AMI, bootstrap, kubelet, and launch template upgrade path manually.

Confidence: `PROVIDER_REPORTED` · Upgrade gate: `allow` · Impact scope: `-` · Can upgrade continue: Yes

> **Why this matters (P4):** Upgrade should not begin while workloads, nodes, or critical add-ons are unhealthy.

**Evidence:**

- launchTemplate: true
- amiType: AL2023_x86_64_STANDARD

**Remediation:**

```
Manually validate the launch template or custom AMI upgrade path, including bootstrap configuration, kubelet version, user data, and AMI release process.
```

## Recommended Maintenance (7)

1. **[P3/Warning] Deployment/kube-system/coredns** (DRAIN-003)

   ```
   Label additional nodes to match this workload's nodeSelector/nodeAffinity (and taint them consistently if tolerations are also required), or relax the constraint if it's broader than actually needed.
   ```

2. **[P3/Warning] Deployment/kube-system/metrics-server** (DRAIN-003)

   ```
   Label additional nodes to match this workload's nodeSelector/nodeAffinity (and taint them consistently if tolerations are also required), or relax the constraint if it's broader than actually needed.
   ```

3. **[P3/Warning] EKSAddon/vpc-cni** (ADDON-002)

   ```
   Verify this EKS managed add-on against provider compatibility metadata before upgrading. Missing catalog coverage is a warning, not proof of incompatibility.
   ```

4. **[P3/Warning] EKSAddon/coredns** (ADDON-002)

   ```
   Verify this EKS managed add-on against provider compatibility metadata before upgrading. Missing catalog coverage is a warning, not proof of incompatibility.
   ```

5. **[P3/Warning] EKSAddon/kube-proxy** (ADDON-002)

   ```
   Verify this EKS managed add-on against provider compatibility metadata before upgrading. Missing catalog coverage is a warning, not proof of incompatibility.
   ```

6. **[P3/Warning] EKSNodegroup/certification-ng** (EKS-NG-002)

   ```
   Review node group capacity and disruption budgets before upgrade. Consider temporarily increasing desired capacity or otherwise creating eviction headroom for the change window.
   ```

7. **[P4/Warning] ValidatingWebhookConfiguration/vpc-resource-validating-webhook** (WH-005)

   ```
   Confirm this webhook genuinely needs to validate/mutate this resource. If not, narrow its rules to exclude it.
   ```

## Evidence Appendix

Every finding's resource identity and fingerprint — cross-reference by fingerprint for waivers/dedup.

| Priority | Rule ID | Severity | Confidence | Resource | Fingerprint |
|---|---|---|---|---|---|
| P3 | DRAIN-003 | Warning | OBSERVED | Deployment/kube-system/coredns | `d5c1e9da1b99cd6c8ece2807daa4285ca2b2751e05c3f9ba3a12a7ad498ff7e5` |
| P3 | DRAIN-003 | Warning | OBSERVED | Deployment/kube-system/metrics-server | `8110f1ce72a44a1d9d399ac5c300fae82318ec4fba1f88595469de3ad1b1421d` |
| P3 | ADDON-002 | Warning | PROVIDER_REPORTED | EKSAddon/coredns | `f63d64bb167c9711816f69c18d4f758599d004df54b6753dcd9338623e75c85d` |
| P3 | ADDON-002 | Warning | PROVIDER_REPORTED | EKSAddon/kube-proxy | `a20104595966e2ff1e1be97019aa86416f4c25a1c11314840f35d21fece049e5` |
| P3 | ADDON-002 | Warning | PROVIDER_REPORTED | EKSAddon/vpc-cni | `70b0ea93b3d57156023dd18b9f73003c0e562d6bcdace8f844787f8658eb3c36` |
| P3 | EKS-NG-002 | Warning | PROVIDER_REPORTED | EKSNodegroup/certification-ng | `eb647ca8a42be4fd791463f0006b95568e9b9e6e9cf7747bf518b54af45c0317` |
| P4 | WH-005 | Warning | STATIC_CERTAIN | ValidatingWebhookConfiguration/vpc-resource-validating-webhook | `a6f358136d35ceaa49be081be84d2ee49cd06dd788f3ed04d2e95530f958629a` |
| P4 | EKS-NG-003 | Info | PROVIDER_REPORTED | EKSNodegroup/certification-ng | `163c437d5a2455893f284e9ceba54406c29e0762368ec89d90ec034d14951e84` |
