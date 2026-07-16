# KubePreflight Scan Report

| | |
|---|---|
| **Cluster** | kubepreflight-case-study |
| **Full cluster identifier** | `arn:aws:eks:us-east-1:000000000000:cluster/kubepreflight-case-study` |
| **Target version** | 1.32 |
| **Provider** | eks |
| **Scanned at** | 2026-07-16 17:00:58 UTC |
| **Result** | **BLOCKED** |
| **Summary** | 7 blocker(s), 11 warning(s), 29 info(s) |

## Upgrade Readiness

| | |
|---|---|
| **Verdict** | BLOCKED |
| **Readiness score** | 19/100 |
| **Upgrade continue** | No |

| Category | Status | Blockers | Warnings | Rule IDs |
|---|---|---|---|---|
| API Compatibility | Failed | 1 | 0 | API-001 |
| Extension APIs | Passed | 0 | 0 |  |
| Admission Webhooks | Failed | 3 | 3 | WH-001, WH-002, WH-004, WH-005 |
| Disruption Safety | Failed | 3 | 0 | PDB-001, PDB-002 |
| Drain Readiness | Warning | 0 | 3 | DRAIN-001, DRAIN-003 |
| Node Readiness | Warning | 0 | 1 | EKS-NG-002, EKS-NG-003, EKS-NG-004 |
| Add-ons | Warning | 0 | 3 | ADDON-002 |
| CoreDNS | Passed | 0 | 0 |  |
| Workload Health | Warning | 0 | 1 | WORKLOAD-001 |
| EKS Upgrade Insights | Passed | 0 | 0 | EKS-INSIGHT-003 |

## API Compatibility

| | |
|---|---|
| **Status** | Failed |
| **Upgrade continue** | No |
| **Score impact** | -25 |
| **Removed API objects** | 1 across 1 API family |
| **Deprecated API objects** | 0 across 0 API families |
| **Critical impact** | No |

### Removed API families

| API version | Kind | Objects |
|---|---|---|
| policy/v1beta1 | PodDisruptionBudget | 1 |

## Blockers (7)

### `P1` `WH-005` ValidatingWebhookConfiguration "dead-fail-closed-webhook": webhook "dead.preflight.local" (index 0 in .webhooks) matches validatingwebhookconfigurations — this webhook can intercept writes to admission webhook configs, including attempts to fix or disable itself

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P1):** May block kubectl apply/patch/scale, Helm upgrades, and controller reconciliation.

**Evidence:**

- webhook name: dead.preflight.local
- matched resource: validatingwebhookconfigurations
- failurePolicy: Fail

**Remediation:**

```
Exclude admissionregistration.k8s.io (validatingwebhookconfigurations/mutatingwebhookconfigurations) from this webhook's rules, so a misbehaving webhook can always be patched or deleted.
```

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

### `P2` `WH-005` ValidatingWebhookConfiguration "dead-fail-closed-webhook": webhook "dead.preflight.local" (index 0 in .webhooks) matches nodes — a fail-closed webhook here can block node status updates, namespace lifecycle, or PersistentVolume operations that upgrade/maintenance workflows depend on

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- webhook name: dead.preflight.local
- matched resource: nodes
- failurePolicy: Fail

**Remediation:**

```
Confirm this webhook genuinely needs to validate/mutate this resource. If not, narrow its rules to exclude it.
```

### `P3` `PDB-001` PodDisruptionBudget preflight-lab/critical-app-pdb: disruptionsAllowed=0 (minAvailable: 1, currentHealthy=0, desiredHealthy=1, expectedPods=1) — healthy matching pods cannot currently be voluntarily evicted, so a node drain or node upgrade can stall or fail

Confidence: `OBSERVED` · Can upgrade continue: No

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- disruptionsAllowed: 0
- minAvailable: 1
- currentHealthy: 0
- desiredHealthy: 1
- expectedPods: 1
- observedGeneration: 1 (metadata.generation: 1)

**Remediation:**

```
Safest-first remediation ladder: (1) scale up replicas to create eviction headroom without changing the PDB contract; (2) add topologySpreadConstraints to distribute the disruption cost across nodes; (3) temporarily relax this PDB for the change window, with an explicit revert step in the change ticket. Force-updating the node group to bypass PDBs is a last resort and must be a recorded business decision, not a default.
```

### `P3` `PDB-001` PodDisruptionBudget preflight-lab/critical-app-pdb-overlap: disruptionsAllowed=0 (minAvailable: 1, currentHealthy=1, desiredHealthy=1, expectedPods=1) — healthy matching pods cannot currently be voluntarily evicted, so a node drain or node upgrade can stall or fail

Confidence: `OBSERVED` · Can upgrade continue: No

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- disruptionsAllowed: 0
- minAvailable: 1
- currentHealthy: 1
- desiredHealthy: 1
- expectedPods: 1
- observedGeneration: 1 (metadata.generation: 1)

**Remediation:**

```
Safest-first remediation ladder: (1) scale up replicas to create eviction headroom without changing the PDB contract; (2) add topologySpreadConstraints to distribute the disruption cost across nodes; (3) temporarily relax this PDB for the change window, with an explicit revert step in the change ticket. Force-updating the node group to bypass PDBs is a last resort and must be a recorded business decision, not a default.
```

### `P3` `PDB-002` PodDisruptionBudgets preflight-lab/critical-app-pdb and preflight-lab/critical-app-pdb-overlap select an overlapping set of pods (1 overlapping: critical-app-648797c4c8-7wjkp) — the Eviction API rejects eviction when multiple PDBs match the same pod, even if each individually would allow disruption

Confidence: `OBSERVED` · Can upgrade continue: No

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- PDB A: preflight-lab/critical-app-pdb (selector: app=critical-app)
- PDB B: preflight-lab/critical-app-pdb-overlap (selector: app=critical-app)
- overlapping pods: critical-app-648797c4c8-7wjkp

**Remediation:**

```
Inspect both PDBs and their owners first. Then delete only a budget confirmed to be duplicate/redundant, or narrow one selector so each pod is selected by at most one PDB. For an AWS-managed CoreDNS PDB collision, confirm ownership before retaining the managed budget and removing the duplicate.
```

### `P4` `WH-002` ValidatingWebhookConfiguration "dead-fail-closed-webhook": webhook "dead.preflight.local" (index 0 in .webhooks) is fail-closed and its backend service preflight-lab/dead-webhook has zero ready endpoints — matching API writes will be rejected

Confidence: `OBSERVED` · Can upgrade continue: No

> **Why this matters (P4):** Upgrade should not begin while workloads, nodes, or critical add-ons are unhealthy.

**Evidence:**

- webhook name: dead.preflight.local
- webhook index: 0
- backend service: preflight-lab/dead-webhook
- ready endpoint address count: 0
- failurePolicy: Fail

**Remediation:**

```
Step 1 — restore the webhook backend (read-only, safe to run any time):

kubectl get svc 'dead-webhook' -n 'preflight-lab'
kubectl get endpointslices -n 'preflight-lab' -l kubernetes.io/service-name='dead-webhook'
kubectl get deploy,pods -n 'preflight-lab'

Step 2 — only if you need immediate relief and cannot wait for the backend to recover:

This TEMPORARILY REMOVES the webhook's protection. The "test" operation
guards against the array index having shifted since this scan ran — the
patch aborts instead of silently touching the wrong webhook block.

kubectl patch validatingwebhookconfiguration 'dead-fail-closed-webhook' --type='json' -p='[{"op":"test","path":"/webhooks/0/name","value":"dead.preflight.local"},{"op":"replace","path":"/webhooks/0/failurePolicy","value":"Ignore"}]'

Revert failurePolicy to Fail immediately after the backend recovers.
```

## Warnings (11)

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

### `P3` `DRAIN-001` Deployment preflight-lab/critical-app runs a single replica (desired: 1, ready: 1, available: 1) — when its pod is evicted (node drain, node upgrade, or any voluntary disruption), this workload has zero available replicas until a replacement schedules and becomes Ready elsewhere; protected by PodDisruptionBudget(s) critical-app-pdb, critical-app-pdb-overlap, which governs whether eviction is currently permitted but does not add replacement capacity

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- desired replicas: 1
- ready replicas: 1
- available replicas: 1
- rollout strategy: RollingUpdate
- PodDisruptionBudget(s): critical-app-pdb, critical-app-pdb-overlap
- affected pod(s): critical-app-648797c4c8-7wjkp

**Remediation:**

```
Increase replicas to create real eviction headroom (a healthy PDB alone doesn't prevent the capacity gap), or explicitly accept single-replica downtime for this workload and document it. If this workload can't run multiple replicas (e.g. a singleton controller with leader election), consider a PodDisruptionBudget with minAvailable: 0 combined with a documented manual coordination process for drains.
```

### `P3` `DRAIN-003` Deployment kube-system/coredns has a nodeSelector/required nodeAffinity satisfied by only 1 node(s) in this cluster today (ip-10-0-1-100.ec2.internal) — if that node is drained, no other currently-known node can host a replacement pod

Confidence: `OBSERVED` · Can upgrade continue: Yes

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- qualifying node(s): ip-10-0-1-100.ec2.internal

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

### `P4` `WH-001` ValidatingWebhookConfiguration "dead-fail-closed-webhook": webhook "dead.preflight.local" is fail-closed with catch-all resource rules (apiGroups: ["*"], resources: ["*/*"], operations: [CREATE,UPDATE]) — requests that also satisfy its configured selectors/match conditions depend on this webhook's backend being healthy

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P4):** Upgrade should not begin while workloads, nodes, or critical add-ons are unhealthy.

**Evidence:**

- webhook name: dead.preflight.local
- scope: apiGroups=["*"], resources=["*/*"]
- operations: [CREATE,UPDATE]
- failurePolicy: Fail
- namespaceSelector set: true
- objectSelector set: false
- matchConditions set: false

**Remediation:**

```
Inspect the webhook's current rules and selectors:

kubectl get validatingwebhookconfiguration 'dead-fail-closed-webhook' -o yaml

Then narrow the webhook's rules to the specific apiGroups/resources it actually needs to
validate/mutate, and add a namespaceSelector excluding kube-system and other critical
namespaces. If this webhook does simple field validation, consider migrating it to a
ValidatingAdmissionPolicy (CEL) to remove the callback dependency entirely.
```

### `P4` `WH-004` ValidatingWebhookConfiguration "dead-fail-closed-webhook": webhook "dead.preflight.local" (index 0 in .webhooks) has no caBundle set — the API server falls back to its system trust roots, which won't validate a self-signed or cluster-internal certificate

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P4):** Upgrade should not begin while workloads, nodes, or critical add-ons are unhealthy.

**Evidence:**

- webhook name: dead.preflight.local
- caBundle: not set
- failurePolicy: Fail

**Remediation:**

```
If the webhook backend uses a self-signed or cluster-internal certificate (the common case for in-cluster webhooks), set clientConfig.caBundle to the CA that signed it. If the backend's certificate is already signed by a system-trusted CA, no action is needed.
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

### `P4` `WORKLOAD-001` Workload has unhealthy pods before upgrade: 1 pod in ErrImagePull. This workload was unhealthy before the upgrade, which can make post-upgrade validation ambiguous.

Confidence: `OBSERVED` · Can upgrade continue: Yes

> **Why this matters (P4):** Upgrade should not begin while workloads, nodes, or critical add-ons are unhealthy.

**Evidence:**

- namespace: preflight-case-study
- pod: already-broken-app-795cc7b4cd-4m4xn
- phase: Pending
- container: nginx
- reason: ErrImagePull
- ready: false
- restartCount: 0

**Remediation:**

```
Inspect the unhealthy workload before the upgrade. Confirm whether this is an expected pre-existing condition or a real application issue. Fix image references, pull secrets, config errors, or failing containers before the change window, or document an explicit waiver in the change ticket.
```

## Info (29)

### `P2` `API-001` FlowSchema "catch-all" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

### `P2` `API-001` FlowSchema "eks-exempt" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

### `P2` `API-001` FlowSchema "eks-leader-election" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

### `P2` `API-001` FlowSchema "eks-monitoring" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

### `P2` `API-001` FlowSchema "eks-workload-high" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

### `P2` `API-001` FlowSchema "endpoint-controller" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

### `P2` `API-001` FlowSchema "exempt" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

### `P2` `API-001` FlowSchema "global-default" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

### `P2` `API-001` FlowSchema "kube-controller-manager" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

### `P2` `API-001` FlowSchema "kube-scheduler" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

### `P2` `API-001` FlowSchema "kube-system-service-accounts" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

### `P2` `API-001` FlowSchema "probes" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

### `P2` `API-001` FlowSchema "service-accounts" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

### `P2` `API-001` FlowSchema "system-leader-election" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

### `P2` `API-001` FlowSchema "system-node-high" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

### `P2` `API-001` FlowSchema "system-nodes" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

### `P2` `API-001` FlowSchema "workload-leader-election" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

### `P2` `API-001` PriorityLevelConfiguration "catch-all" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

### `P2` `API-001` PriorityLevelConfiguration "eks-monitoring" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

### `P2` `API-001` PriorityLevelConfiguration "exempt" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

### `P2` `API-001` PriorityLevelConfiguration "global-default" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

### `P2` `API-001` PriorityLevelConfiguration "leader-election" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

### `P2` `API-001` PriorityLevelConfiguration "node-high" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

### `P2` `API-001` PriorityLevelConfiguration "system" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

### `P2` `API-001` PriorityLevelConfiguration "workload-high" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

### `P2` `API-001` PriorityLevelConfiguration "workload-low" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta3
- removed in: Kubernetes 1.32
- target version: 1.32
- detected via: live cluster object
- reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or a platform field manager such as EKS's own eks-internal (cloud-provider-injected default)
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#flowcontrol-resources-v132

**Remediation:**

```
No action needed for this specific object: kube-apiserver or the cloud provider's control plane owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only recognized bootstrap sets (kube-apiserver's own, marked with the apf.kubernetes.io/autoupdate-spec annotation, and known cloud-provider ones) are covered by this note.
```

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

## Next Actions (12)

1. **[P4/Blocker] ValidatingWebhookConfiguration/dead-fail-closed-webhook** (WH-001, WH-002, WH-004, WH-005, WH-005)

   ```
   Step 1 — restore the webhook backend (read-only, safe to run any time):

   kubectl get svc 'dead-webhook' -n 'preflight-lab'
   kubectl get endpointslices -n 'preflight-lab' -l kubernetes.io/service-name='dead-webhook'
   kubectl get deploy,pods -n 'preflight-lab'

   Step 2 — only if you need immediate relief and cannot wait for the backend to recover:

   This TEMPORARILY REMOVES the webhook's protection. The "test" operation
   guards against the array index having shifted since this scan ran — the
   patch aborts instead of silently touching the wrong webhook block.

   kubectl patch validatingwebhookconfiguration 'dead-fail-closed-webhook' --type='json' -p='[{"op":"test","path":"/webhooks/0/name","value":"dead.preflight.local"},{"op":"replace","path":"/webhooks/0/failurePolicy","value":"Ignore"}]'

   Revert failurePolicy to Fail immediately after the backend recovers.
   ```

   Also see `WH-005`: Exclude admissionregistration.k8s.io (validatingwebhookconfigurations/mutatingwebhookconfigurations) from this webhook's rules, so a misbehaving webhook can always be patched or deleted.

   Also see `WH-005`: Confirm this webhook genuinely needs to validate/mutate this resource. If not, narrow its rules to exclude it.

   Also see `WH-001`: Inspect the webhook's current rules and selectors: ...

   Also see `WH-004`: If the webhook backend uses a self-signed or cluster-internal certificate (the common case for in-cluster webhooks), set clientConfig.caBundle to the CA that signed it. If the backend's certificate is already signed by a system-trusted CA, no action is needed.

2. **[P2/Blocker] PodDisruptionBudget/default/old-pdb-api** (API-001)

   ```
   Migrate to policy/v1 PodDisruptionBudget before this manifest is ever applied to a cluster at or past 1.25. Update and validate the source manifest against the replacement schema. For Helm charts, update the template itself — bumping the chart version alone doesn't help if the template source still emits the old apiVersion.
   ```

3. **[P3/Blocker] PodDisruptionBudget/preflight-lab/critical-app-pdb** (PDB-001, PDB-001, PDB-002)

   ```
   Safest-first remediation ladder: (1) scale up replicas to create eviction headroom without changing the PDB contract; (2) add topologySpreadConstraints to distribute the disruption cost across nodes; (3) temporarily relax this PDB for the change window, with an explicit revert step in the change ticket. Force-updating the node group to bypass PDBs is a last resort and must be a recorded business decision, not a default.
   ```

   Also see `PDB-002`: Inspect both PDBs and their owners first. Then delete only a budget confirmed to be duplicate/redundant, or narrow one selector so each pod is selected by at most one PDB. For an AWS-managed CoreDNS PDB collision, confirm ownership before retaining the managed budget and removing the duplicate.

4. **[P3/Warning] Deployment/preflight-case-study/already-broken-app** (DRAIN-001)

   ```
   Increase replicas to create real eviction headroom (a healthy PDB alone doesn't prevent the capacity gap), or explicitly accept single-replica downtime for this workload and document it. If this workload can't run multiple replicas (e.g. a singleton controller with leader election), consider a PodDisruptionBudget with minAvailable: 0 combined with a documented manual coordination process for drains.
   ```

5. **[P3/Warning] Deployment/preflight-lab/critical-app** (DRAIN-001)

   ```
   Increase replicas to create real eviction headroom (a healthy PDB alone doesn't prevent the capacity gap), or explicitly accept single-replica downtime for this workload and document it. If this workload can't run multiple replicas (e.g. a singleton controller with leader election), consider a PodDisruptionBudget with minAvailable: 0 combined with a documented manual coordination process for drains.
   ```

6. **[P3/Warning] Deployment/kube-system/coredns** (DRAIN-003)

   ```
   Label additional nodes to match this workload's nodeSelector/nodeAffinity (and taint them consistently if tolerations are also required), or relax the constraint if it's broader than actually needed.
   ```

7. **[P3/Warning] EKSAddon/vpc-cni** (ADDON-002)

   ```
   Verify this EKS managed add-on against provider compatibility metadata before upgrading. Missing catalog coverage is a warning, not proof of incompatibility.
   ```

8. **[P3/Warning] EKSNodegroup/ng-small** (EKS-NG-002)

   ```
   Review node group capacity and disruption budgets before upgrade. Consider temporarily increasing desired capacity or otherwise creating eviction headroom for the change window.
   ```

9. **[P3/Warning] EKSAddon/coredns** (ADDON-002)

   ```
   Verify this EKS managed add-on against provider compatibility metadata before upgrading. Missing catalog coverage is a warning, not proof of incompatibility.
   ```

10. **[P3/Warning] EKSAddon/kube-proxy** (ADDON-002)

   ```
   Verify this EKS managed add-on against provider compatibility metadata before upgrading. Missing catalog coverage is a warning, not proof of incompatibility.
   ```

11. **[P4/Warning] ValidatingWebhookConfiguration/vpc-resource-validating-webhook** (WH-005)

   ```
   Confirm this webhook genuinely needs to validate/mutate this resource. If not, narrow its rules to exclude it.
   ```

12. **[P4/Warning] Pod/preflight-case-study/already-broken-app-795cc7b4cd-4m4xn** (WORKLOAD-001)

   ```
   Inspect the unhealthy workload before the upgrade. Confirm whether this is an expected pre-existing condition or a real application issue. Fix image references, pull secrets, config errors, or failing containers before the change window, or document an explicit waiver in the change ticket.
   ```

## Evidence Appendix

Every finding's resource identity and fingerprint — cross-reference by fingerprint for waivers/dedup.

| Priority | Rule ID | Severity | Confidence | Resource | Fingerprint |
|---|---|---|---|---|---|
| P1 | WH-005 | Blocker | STATIC_CERTAIN | ValidatingWebhookConfiguration/dead-fail-closed-webhook | `63cae4e1459da1ee273f2099582b9f226719f34adb1022004946ab2460d8b689` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | PodDisruptionBudget/default/old-pdb-api | `81ec7e435063b6a876acb5d995390a419db2fd8d56bf6965a1a5c4e36914da38` |
| P2 | WH-005 | Blocker | STATIC_CERTAIN | ValidatingWebhookConfiguration/dead-fail-closed-webhook | `279fba496054f118c20c350e23c83298184b3a85253d9a68d63a2c7dda152860` |
| P2 | API-001 | Info | STATIC_CERTAIN | FlowSchema/catch-all | `50627df11ced84a35b9a1d677d8ba5997f8e212e12f3a5d5138c578b677c9389` |
| P2 | API-001 | Info | STATIC_CERTAIN | FlowSchema/eks-exempt | `982f1f3003baf1142214a9d31c7f8b85f8c11a1f6dd4a010154286e399ddac1f` |
| P2 | API-001 | Info | STATIC_CERTAIN | FlowSchema/eks-leader-election | `db3fd489a914e41eb35ea314fe8421ce298c29cd34b10d0d57de5930d6f20b76` |
| P2 | API-001 | Info | STATIC_CERTAIN | FlowSchema/eks-monitoring | `2fd1796b5d586a988cd2026aa7671f4c4d3dbbaf0d3d0afddb83ccef629438de` |
| P2 | API-001 | Info | STATIC_CERTAIN | FlowSchema/eks-workload-high | `1aa957966a3ed19dc1b30b6718f2f07f50118241c42895e5794bdcfe477d1143` |
| P2 | API-001 | Info | STATIC_CERTAIN | FlowSchema/endpoint-controller | `a2340ac6d74fa12914ae26b707d3d579080bd6de1180259d5928c55be91c45a8` |
| P2 | API-001 | Info | STATIC_CERTAIN | FlowSchema/exempt | `1d364d81a183907d4f3073dea8a5fceaf3e47000da864dedea58005f9aeed407` |
| P2 | API-001 | Info | STATIC_CERTAIN | FlowSchema/global-default | `eefcceae3185c182201a6bfde7ae05e8d8d9c78ba0e788c23c2def4f68ec374c` |
| P2 | API-001 | Info | STATIC_CERTAIN | FlowSchema/kube-controller-manager | `f35262de93f297d2fc4d8e9467dec8daa9f2b891263f626b3721135a42106051` |
| P2 | API-001 | Info | STATIC_CERTAIN | FlowSchema/kube-scheduler | `543f5527abb1df00c43d96720ff6b3d5e6160e8be948c779d73b520f6c22c516` |
| P2 | API-001 | Info | STATIC_CERTAIN | FlowSchema/kube-system-service-accounts | `f57d79f6541c001ada5ff756518d0d3177d697c3157c7ee91c908183d315b131` |
| P2 | API-001 | Info | STATIC_CERTAIN | FlowSchema/probes | `78b46b3fdb46dc5bc69a78662d953526118aa02bcfc91c94e7cf9b6a8d841a02` |
| P2 | API-001 | Info | STATIC_CERTAIN | FlowSchema/service-accounts | `a698b0b3327de68d5766cd4ea4812267fa534ccdf0ba0644045e254287bf09a4` |
| P2 | API-001 | Info | STATIC_CERTAIN | FlowSchema/system-leader-election | `1e5dd416126aac5acf39308aab0768bc7d07a12c0936171db6b0ac0a5c1b8267` |
| P2 | API-001 | Info | STATIC_CERTAIN | FlowSchema/system-node-high | `e2ca68fac043eff3445f5352e8a2fa8ba17f1bccbe22b29d2d8fb3f2720738aa` |
| P2 | API-001 | Info | STATIC_CERTAIN | FlowSchema/system-nodes | `b9bca7c66c24cf0878f88ce6db173d49ea93e83ec515ce0e4df5af56b8bb44f8` |
| P2 | API-001 | Info | STATIC_CERTAIN | FlowSchema/workload-leader-election | `f368f3598a4357d89e63e5845630b65fb2e6ccae02165a420e79c2e9c2864408` |
| P2 | API-001 | Info | STATIC_CERTAIN | PriorityLevelConfiguration/catch-all | `50cde586c679b6a72365fd0019b5797fef66b71f28c6297d9c64c9aba9eda8ba` |
| P2 | API-001 | Info | STATIC_CERTAIN | PriorityLevelConfiguration/eks-monitoring | `1a0b608bad10e286980beb1521bd2de53b19f1f5603f6a655ba9a2935e761d72` |
| P2 | API-001 | Info | STATIC_CERTAIN | PriorityLevelConfiguration/exempt | `55027d2e56a22e6c7e6992915ddcbf2988e9913b383bac3a7e0b81823debdb89` |
| P2 | API-001 | Info | STATIC_CERTAIN | PriorityLevelConfiguration/global-default | `650aa967fd264cd6f99023f082a6affa926e0d430aec4e4cc1a3483fd3723c99` |
| P2 | API-001 | Info | STATIC_CERTAIN | PriorityLevelConfiguration/leader-election | `1f909dbffa4ee777a1af5ed3ed1cc1ce8f6e7daed14ae15a9a45019edfab4ad9` |
| P2 | API-001 | Info | STATIC_CERTAIN | PriorityLevelConfiguration/node-high | `d955f3ed1d5e2a2689fcd44ee3abb8606551efdb96cfa147dadd52e9c42345a5` |
| P2 | API-001 | Info | STATIC_CERTAIN | PriorityLevelConfiguration/system | `365101345a73b0a3cb56378fc814364e26e930f2d26fb94555dead10bc0f7708` |
| P2 | API-001 | Info | STATIC_CERTAIN | PriorityLevelConfiguration/workload-high | `e37592ea00d037ffc1dfccfda3bb39f9fdba900f8968713f41c75cd20de62f35` |
| P2 | API-001 | Info | STATIC_CERTAIN | PriorityLevelConfiguration/workload-low | `fb4ab0679dcb580ce9615f8bd75c1f0da06de24044c7bc27724049c8ad1e4442` |
| P3 | PDB-001 | Blocker | OBSERVED | PodDisruptionBudget/preflight-lab/critical-app-pdb | `0dbda67837cff638382166e61e92a2b44eabf5669465a0175ca49fdb764d20b1` |
| P3 | PDB-001 | Blocker | OBSERVED | PodDisruptionBudget/preflight-lab/critical-app-pdb-overlap | `d3a220474f72425e028d49b278d6c619444056c1b216fe0a8cd7225f13ee8d13` |
| P3 | PDB-002 | Blocker | OBSERVED | PodDisruptionBudget/preflight-lab/critical-app-pdb,critical-app-pdb-overlap | `6c7e64425f771bd4f37d237d6957a0aba3b20dfe17c24bb079639c0b31e5c643` |
| P3 | DRAIN-001 | Warning | STATIC_CERTAIN | Deployment/preflight-case-study/already-broken-app | `39bbf124d658a44e35eab4386a23c3323f1d79f05e0759bf555ad54a6f60d461` |
| P3 | DRAIN-001 | Warning | STATIC_CERTAIN | Deployment/preflight-lab/critical-app | `7b5b8d23a52de27ff7d3a11510668ade515baf023e867c791e877aa36cdb8edb` |
| P3 | DRAIN-003 | Warning | OBSERVED | Deployment/kube-system/coredns | `9925e4909da35425eafa4ba590eebf6b0ccab6130e218da755c017e8a96cbfcb` |
| P3 | ADDON-002 | Warning | PROVIDER_REPORTED | EKSAddon/coredns | `adfaf920d7a0783f9c7444955b597586b8ce7d1333ce7fee55df929d3b355a5b` |
| P3 | ADDON-002 | Warning | PROVIDER_REPORTED | EKSAddon/kube-proxy | `3f0311cbf087fa5c4637299961cf173c23dc3f7477a6a67ce74ddb4267221a6e` |
| P3 | ADDON-002 | Warning | PROVIDER_REPORTED | EKSAddon/vpc-cni | `1daa7c588e5cef48fafc8922ae1c5d1503ce39c517547263fab228767f3626cf` |
| P3 | EKS-NG-002 | Warning | PROVIDER_REPORTED | EKSNodegroup/ng-small | `62d9fabecaad44e09470cd9ace240159d487c2b3792a6777b5179807097a8803` |
| P4 | WH-002 | Blocker | OBSERVED | ValidatingWebhookConfiguration/dead-fail-closed-webhook | `1e6947338ec3273186b84e2c62751463d2139ab5dc061b422fedb4bc0536a6b7` |
| P4 | WH-001 | Warning | STATIC_CERTAIN | ValidatingWebhookConfiguration/dead-fail-closed-webhook | `82329893508ede1ca07692878aa8e4e76e135195e3df256966166d1874bae0ca` |
| P4 | WH-004 | Warning | STATIC_CERTAIN | ValidatingWebhookConfiguration/dead-fail-closed-webhook | `a238a5e8d1570f194da2105244548e411dbfb474cad06d065d9eadbe919a807c` |
| P4 | WH-005 | Warning | STATIC_CERTAIN | ValidatingWebhookConfiguration/vpc-resource-validating-webhook | `8820528e4bab3c5bf05ec85d24e22d68d36e9022a531f6f776752e4af6fee560` |
| P4 | WORKLOAD-001 | Warning | OBSERVED | Pod/preflight-case-study/already-broken-app-795cc7b4cd-4m4xn | `e8cae36138ee8845c05a4edf3f033ff4eaaa0cb7eba2076ae65c4b7c684fb73b` |
| P4 | EKS-INSIGHT-003 | Info | PROVIDER_REPORTED | EKSUpgradeInsight/EKS add-on version compatibility | `11cecc33d7ab34bc2487c1c77956184382f5254665a78c95843d2b075db95cd6` |
| P4 | EKS-NG-003 | Info | PROVIDER_REPORTED | EKSNodegroup/ng-small | `493e7122924cfac215c2d071bf09c897dca126030f84be2c8f58926baeb0651e` |
| P4 | EKS-NG-004 | Info | PROVIDER_REPORTED | EKSNodegroup/ng-small | `e2fd5e6a2e3696684526781f6f322ee075a9933ba76073bb1dd1286c2915fcd6` |

