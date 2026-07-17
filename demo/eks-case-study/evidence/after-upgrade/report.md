# KubePreflight Scan Report

| | |
|---|---|
| **Cluster** | kubepreflight-case-study |
| **Full cluster identifier** | `[redacted cluster ARN]` |
| **Target version** | 1.32 |
| **Provider** | eks |
| **Scanned at** | 2026-07-16 17:14:16 UTC |
| **Result** | **BLOCKED** |
| **Summary** | 1 blocker(s), 8 warning(s), 3 info(s) |

> **No version upgrade required:** cluster is already running Kubernetes 1.32 (target: 1.32). Upgrade-transition checks were skipped; current-state and manifest-safety findings below were still fully evaluated.

## Cluster Health (no version upgrade assessed)

| | |
|---|---|
| **Verdict** | BLOCKED |
| **Readiness score** | 57/100 |
| **Remediation needed** | Yes |

| Category | Status | Blockers | Warnings | Rule IDs |
|---|---|---|---|---|
| API Compatibility | Failed | 1 | 0 | API-001 |
| Extension APIs | Passed | 0 | 0 |  |
| Admission Webhooks | Warning | 0 | 1 | WH-005 |
| Disruption Safety | Passed | 0 | 0 |  |
| Drain Readiness | Warning | 0 | 2 | DRAIN-001, DRAIN-003 |
| Node Readiness | Warning | 0 | 1 | EKS-NG-002, EKS-NG-003, EKS-NG-004 |
| Add-ons | Warning | 0 | 3 | ADDON-002 |
| CoreDNS | Passed | 0 | 0 |  |
| Workload Health | Warning | 0 | 1 | WORKLOAD-001 |
| EKS Upgrade Insights | Passed | 0 | 0 | EKS-INSIGHT-003 |

## API Compatibility

| | |
|---|---|
| **Status** | Failed |
| **Remediation needed** | Yes |
| **Score impact** | -25 |
| **Removed API objects** | 1 across 1 API family |
| **Deprecated API objects** | 0 across 0 API families |
| **Critical impact** | No |

### Removed API families

| API version | Kind | Objects |
|---|---|---|
| policy/v1beta1 | PodDisruptionBudget | 1 |

## Blockers (1)

### `P2` `API-001` PodDisruptionBudget "default/old-pdb-api" (apiVersion policy/v1beta1) in old-api.yaml uses an API version removed in Kubernetes 1.25 — this manifest will fail to apply once the cluster reaches target 1.32

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: policy/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.32
- source: old-api.yaml
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#poddisruptionbudget-v125

**Remediation:**

```
Migrate to policy/v1 PodDisruptionBudget before this manifest is ever applied to a cluster at or past 1.25. Update and validate the source manifest against the replacement schema. For Helm charts, update the template itself — bumping the chart version alone doesn't help if the template source still emits the old apiVersion.
```

## Warnings (8)

### `P3` `DRAIN-001` Deployment preflight-case-study/already-broken-app runs a single replica (desired: 1, ready: 0, available: 0) — when its pod is evicted (node drain, node upgrade, or any voluntary disruption), this workload has zero available replicas until a replacement schedules and becomes Ready elsewhere; no PodDisruptionBudget protects this workload

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- desired replicas: 1
- ready replicas: 0
- available replicas: 0
- rollout strategy: RollingUpdate
- PodDisruptionBudget(s): none
- affected pod(s): already-broken-app-795cc7b4cd-4m4xn

**Remediation:**

```
Increase replicas to create real eviction headroom (a healthy PDB alone doesn't prevent the capacity gap), or explicitly accept single-replica downtime for this workload and document it. If this workload can't run multiple replicas (e.g. a singleton controller with leader election), consider a PodDisruptionBudget with minAvailable: 0 combined with a documented manual coordination process for drains.
```

### `P3` `DRAIN-003` Deployment kube-system/coredns has a nodeSelector/required nodeAffinity satisfied by only 1 node(s) in this cluster today ([redacted node hostname]) — if that node is drained, no other currently-known node can host a replacement pod

Confidence: `OBSERVED` · Can upgrade continue: Yes

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- qualifying node(s): [redacted node hostname]

**Remediation:**

```
Label additional nodes to match this workload's nodeSelector/nodeAffinity (and taint them consistently if tolerations are also required), or relax the constraint if it's broader than actually needed.
```

### `P3` `ADDON-002` EKS add-on "coredns" version v1.11.4-eksbuild.39 has no compatibility catalog entry for target Kubernetes 1.32 — confirm compatibility before starting the upgrade

Confidence: `PROVIDER_REPORTED` · Can upgrade continue: Yes

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- installed add-on: coredns
- current version: v1.11.4-eksbuild.39
- target Kubernetes version: 1.32
- minimum compatible version: unknown
- recommended upgrade version: unknown
- compatibility status: unknown
- catalog source: no catalog entry for provider=eks add-on target
- required upgrade order: 3. CoreDNS after VPC CNI and kube-proxy, before storage CSI add-ons

**Remediation:**

```
Verify this EKS managed add-on against provider compatibility metadata before upgrading. Missing catalog coverage is a warning, not proof of incompatibility.
```

### `P3` `ADDON-002` EKS add-on "kube-proxy" version v1.31.14-eksbuild.18 has no compatibility catalog entry for target Kubernetes 1.32 — confirm compatibility before starting the upgrade

Confidence: `PROVIDER_REPORTED` · Can upgrade continue: Yes

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- installed add-on: kube-proxy
- current version: v1.31.14-eksbuild.18
- target Kubernetes version: 1.32
- minimum compatible version: unknown
- recommended upgrade version: unknown
- compatibility status: unknown
- catalog source: no catalog entry for provider=eks add-on target
- required upgrade order: 2. kube-proxy after VPC CNI and before CoreDNS/storage add-ons

**Remediation:**

```
Verify this EKS managed add-on against provider compatibility metadata before upgrading. Missing catalog coverage is a warning, not proof of incompatibility.
```

### `P3` `ADDON-002` EKS add-on "vpc-cni" version v1.21.2-eksbuild.2 has no compatibility catalog entry for target Kubernetes 1.32 — confirm compatibility before starting the upgrade

Confidence: `PROVIDER_REPORTED` · Can upgrade continue: Yes

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- installed add-on: vpc-cni
- current version: v1.21.2-eksbuild.2
- target Kubernetes version: 1.32
- minimum compatible version: unknown
- recommended upgrade version: unknown
- compatibility status: unknown
- catalog source: no catalog entry for provider=eks add-on target
- required upgrade order: 1. Amazon VPC CNI before kube-proxy and DNS/storage add-ons

**Remediation:**

```
Verify this EKS managed add-on against provider compatibility metadata before upgrading. Missing catalog coverage is a warning, not proof of incompatibility.
```

### `P3` `EKS-NG-002` Managed node group "ng-small" desired size equals or is below minimum size. Rolling update may have limited disruption headroom.

Confidence: `PROVIDER_REPORTED` · Can upgrade continue: Yes

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- desiredSize: 1
- minSize: 1
- maxSize: 1

**Remediation:**

```
Review node group capacity and disruption budgets before upgrade. Consider temporarily increasing desired capacity or otherwise creating eviction headroom for the change window.
```

### `P4` `WH-005` ValidatingWebhookConfiguration "vpc-resource-validating-webhook": webhook "vnode.vpc.k8s.aws" (index 1 in .webhooks) matches nodes — a fail-closed webhook here can block node status updates, namespace lifecycle, or PersistentVolume operations that upgrade/maintenance workflows depend on

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P4):** Upgrade should not begin while workloads, nodes, or critical add-ons are unhealthy.

**Evidence:**

- webhook name: vnode.vpc.k8s.aws
- matched resource: nodes
- failurePolicy: Ignore

**Remediation:**

```
Confirm this webhook genuinely needs to validate/mutate this resource. If not, narrow its rules to exclude it.
```

### `P4` `WORKLOAD-001` Workload has unhealthy pods before upgrade: 1 pod in ImagePullBackOff. This workload was unhealthy before the upgrade, which can make post-upgrade validation ambiguous.

Confidence: `OBSERVED` · Can upgrade continue: Yes

> **Why this matters (P4):** Upgrade should not begin while workloads, nodes, or critical add-ons are unhealthy.

**Evidence:**

- namespace: preflight-case-study
- pod: already-broken-app-795cc7b4cd-4m4xn
- phase: Pending
- container: nginx
- reason: ImagePullBackOff
- ready: false
- restartCount: 0

**Remediation:**

```
Inspect the unhealthy workload before the upgrade. Confirm whether this is an expected pre-existing condition or a real application issue. Fix image references, pull secrets, config errors, or failing containers before the change window, or document an explicit waiver in the change ticket.
```

## Info (3)

### `P4` `EKS-INSIGHT-003` EKS upgrade insight "EKS add-on version compatibility" reports UNKNOWN for Kubernetes 1.32. Treat this as AWS-native context and verify with a fresh scan before upgrade.

Confidence: `PROVIDER_REPORTED` · Can upgrade continue: Yes

> **Why this matters (P4):** Upgrade should not begin while workloads, nodes, or critical add-ons are unhealthy.

**Evidence:**

- insight id: 872ce36f-b68c-41ca-80be-1ab67e5ef49c
- status: UNKNOWN
- kubernetes version: 1.32
- reason: Unable to determine version compatibility of EKS add-ons.
- last refreshed: 2026-07-16T10:49:05Z
- last transition: 2026-07-16T10:49:04Z
- recommendation: Upgrade your EKS add-on to a newer version compatible with the next Kubernetes version.
- add-on compatibility detail: kube-proxy compatible versions: v1.31.14-eksbuild.18, v1.31.14-eksbuild.20, v1.32.0-eksbuild.2, v1.32.3-eksbuild.2, v1.32.3-eksbuild.7, v1.32.5-eksbuild.2, v1.32.6-eksbuild.2, v1.32.6-eksbuild.6, v1.32.6-eksbuild.8, v1.32.6-eksbuild.12, v1.32.9-eksbuild.2, v1.32.11-eksbuild.2, v1.32.11-eksbuild.5, v1.32.13-eksbuild.2, v1.32.13-eksbuild.5, v1.32.13-eksbuild.8, v1.32.13-eksbuild.11, v1.32.13-eksbuild.14, v1.32.13-eksbuild.16
- add-on compatibility detail: vpc-cni compatible versions: v1.21.2-eksbuild.2, v1.22.1-eksbuild.2, v1.22.2-eksbuild.1, v1.22.3-eksbuild.1
- freshness note: AWS-native upgrade readiness checks from Amazon EKS. Insights may be up to 24 hours old; re-check after remediation.

**Remediation:**

```
Upgrade your EKS add-on to a newer version compatible with the next Kubernetes version.

AWS-native upgrade readiness checks from Amazon EKS. Insights may be up to 24 hours old; re-check after remediation.
```

### `P4` `EKS-NG-003` Managed node group "ng-small" uses a launch template/custom AMI. Validate AMI, bootstrap, kubelet, and launch template upgrade path manually.

Confidence: `PROVIDER_REPORTED` · Can upgrade continue: Yes

> **Why this matters (P4):** Upgrade should not begin while workloads, nodes, or critical add-ons are unhealthy.

**Evidence:**

- launchTemplate: true
- amiType: AL2023_x86_64_STANDARD

**Remediation:**

```
Manually validate the launch template or custom AMI upgrade path, including bootstrap configuration, kubelet version, user data, and AMI release process.
```

### `P4` `EKS-NG-004` Managed node group "ng-small" reports Kubernetes version 1.31 while target is 1.32. Node kubelet skew is evaluated separately by NODE-001.

Confidence: `PROVIDER_REPORTED` · Can upgrade continue: Yes

> **Why this matters (P4):** Upgrade should not begin while workloads, nodes, or critical add-ons are unhealthy.

**Evidence:**

- node group Kubernetes version: 1.31
- target Kubernetes version: 1.32
- NODE-001 evaluates actual Kubernetes node/kubelet skew separately.

**Remediation:**

```
Use this as provider inventory context. Confirm actual node kubelet skew in NODE-001 findings and update managed node groups in the provider-recommended sequence.
```

## Recommended Maintenance (9)

1. **[P2/Blocker] PodDisruptionBudget/default/old-pdb-api** (API-001)

   ```
   Migrate to policy/v1 PodDisruptionBudget before this manifest is ever applied to a cluster at or past 1.25. Update and validate the source manifest against the replacement schema. For Helm charts, update the template itself — bumping the chart version alone doesn't help if the template source still emits the old apiVersion.
   ```

2. **[P3/Warning] Deployment/preflight-case-study/already-broken-app** (DRAIN-001)

   ```
   Increase replicas to create real eviction headroom (a healthy PDB alone doesn't prevent the capacity gap), or explicitly accept single-replica downtime for this workload and document it. If this workload can't run multiple replicas (e.g. a singleton controller with leader election), consider a PodDisruptionBudget with minAvailable: 0 combined with a documented manual coordination process for drains.
   ```

3. **[P3/Warning] Deployment/kube-system/coredns** (DRAIN-003)

   ```
   Label additional nodes to match this workload's nodeSelector/nodeAffinity (and taint them consistently if tolerations are also required), or relax the constraint if it's broader than actually needed.
   ```

4. **[P3/Warning] EKSAddon/vpc-cni** (ADDON-002)

   ```
   Verify this EKS managed add-on against provider compatibility metadata before upgrading. Missing catalog coverage is a warning, not proof of incompatibility.
   ```

5. **[P3/Warning] EKSNodegroup/ng-small** (EKS-NG-002)

   ```
   Review node group capacity and disruption budgets before upgrade. Consider temporarily increasing desired capacity or otherwise creating eviction headroom for the change window.
   ```

6. **[P3/Warning] EKSAddon/coredns** (ADDON-002)

   ```
   Verify this EKS managed add-on against provider compatibility metadata before upgrading. Missing catalog coverage is a warning, not proof of incompatibility.
   ```

7. **[P3/Warning] EKSAddon/kube-proxy** (ADDON-002)

   ```
   Verify this EKS managed add-on against provider compatibility metadata before upgrading. Missing catalog coverage is a warning, not proof of incompatibility.
   ```

8. **[P4/Warning] ValidatingWebhookConfiguration/vpc-resource-validating-webhook** (WH-005)

   ```
   Confirm this webhook genuinely needs to validate/mutate this resource. If not, narrow its rules to exclude it.
   ```

9. **[P4/Warning] Pod/preflight-case-study/already-broken-app-795cc7b4cd-4m4xn** (WORKLOAD-001)

   ```
   Inspect the unhealthy workload before the upgrade. Confirm whether this is an expected pre-existing condition or a real application issue. Fix image references, pull secrets, config errors, or failing containers before the change window, or document an explicit waiver in the change ticket.
   ```

## Evidence Appendix

Every finding's resource identity and fingerprint — cross-reference by fingerprint for waivers/dedup.

| Priority | Rule ID | Severity | Confidence | Resource | Fingerprint |
|---|---|---|---|---|---|
| P2 | API-001 | Blocker | STATIC_CERTAIN | PodDisruptionBudget/default/old-pdb-api | `81ec7e435063b6a876acb5d995390a419db2fd8d56bf6965a1a5c4e36914da38` |
| P3 | DRAIN-001 | Warning | STATIC_CERTAIN | Deployment/preflight-case-study/already-broken-app | `39bbf124d658a44e35eab4386a23c3323f1d79f05e0759bf555ad54a6f60d461` |
| P3 | DRAIN-003 | Warning | OBSERVED | Deployment/kube-system/coredns | `9925e4909da35425eafa4ba590eebf6b0ccab6130e218da755c017e8a96cbfcb` |
| P3 | ADDON-002 | Warning | PROVIDER_REPORTED | EKSAddon/coredns | `adfaf920d7a0783f9c7444955b597586b8ce7d1333ce7fee55df929d3b355a5b` |
| P3 | ADDON-002 | Warning | PROVIDER_REPORTED | EKSAddon/kube-proxy | `3f0311cbf087fa5c4637299961cf173c23dc3f7477a6a67ce74ddb4267221a6e` |
| P3 | ADDON-002 | Warning | PROVIDER_REPORTED | EKSAddon/vpc-cni | `1daa7c588e5cef48fafc8922ae1c5d1503ce39c517547263fab228767f3626cf` |
| P3 | EKS-NG-002 | Warning | PROVIDER_REPORTED | EKSNodegroup/ng-small | `62d9fabecaad44e09470cd9ace240159d487c2b3792a6777b5179807097a8803` |
| P4 | WH-005 | Warning | STATIC_CERTAIN | ValidatingWebhookConfiguration/vpc-resource-validating-webhook | `8820528e4bab3c5bf05ec85d24e22d68d36e9022a531f6f776752e4af6fee560` |
| P4 | WORKLOAD-001 | Warning | OBSERVED | Pod/preflight-case-study/already-broken-app-795cc7b4cd-4m4xn | `e8cae36138ee8845c05a4edf3f033ff4eaaa0cb7eba2076ae65c4b7c684fb73b` |
| P4 | EKS-INSIGHT-003 | Info | PROVIDER_REPORTED | EKSUpgradeInsight/EKS add-on version compatibility | `11cecc33d7ab34bc2487c1c77956184382f5254665a78c95843d2b075db95cd6` |
| P4 | EKS-NG-003 | Info | PROVIDER_REPORTED | EKSNodegroup/ng-small | `493e7122924cfac215c2d071bf09c897dca126030f84be2c8f58926baeb0651e` |
| P4 | EKS-NG-004 | Info | PROVIDER_REPORTED | EKSNodegroup/ng-small | `e2fd5e6a2e3696684526781f6f322ee075a9933ba76073bb1dd1286c2915fcd6` |

