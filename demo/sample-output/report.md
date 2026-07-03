# KubePreflight Scan Report

| | |
|---|---|
| **Cluster** | kind-kubepreflight-demo |
| **Target version** | 1.34 |
| **Provider** | cluster-only |
| **Scanned at** | 2026-07-03 17:11:33 UTC |
| **Result** | **BLOCKED** |
| **Summary** | 9 blocker(s), 2 warning(s), 0 info(s) |

## Blockers (9)

### `API-001` PodDisruptionBudget "demo/shared-app-pdb-a" (apiVersion policy/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN`

**Evidence:**

- apiVersion: policy/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to policy/v1 PodDisruptionBudget before upgrading past 1.25. For manifests: `kubectl convert -f <file> --output-version <group>/<version>`. For Helm releases whose stored release manifest still references policy/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `API-001` PodDisruptionBudget "demo/shared-app-pdb-b" (apiVersion policy/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN`

**Evidence:**

- apiVersion: policy/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to policy/v1 PodDisruptionBudget before upgrading past 1.25. For manifests: `kubectl convert -f <file> --output-version <group>/<version>`. For Helm releases whose stored release manifest still references policy/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `API-001` PodDisruptionBudget "demo/singleton-app-pdb" (apiVersion policy/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN`

**Evidence:**

- apiVersion: policy/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to policy/v1 PodDisruptionBudget before upgrading past 1.25. For manifests: `kubectl convert -f <file> --output-version <group>/<version>`. For Helm releases whose stored release manifest still references policy/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `API-001` PodSecurityPolicy "demo-restricted" (apiVersion policy/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN`

**Evidence:**

- apiVersion: policy/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to Pod Security Admission or a policy engine (Kyverno/Gatekeeper) before upgrading past 1.25. For manifests: `kubectl convert -f <file> --output-version <group>/<version>`. For Helm releases whose stored release manifest still references policy/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `NODE-001` Node "kubepreflight-demo-control-plane": kubelet version v1.24.15 is outside the supported skew window for target version 1.34 — kubelet minor version 24 is 10 minor versions behind target minor version 34 — exceeds the supported n-3 skew policy

Confidence: `STATIC_CERTAIN`

**Evidence:**

- kubelet version: v1.24.15
- target version: 1.34
- kubelet minor version 24 is 10 minor versions behind target minor version 34 — exceeds the supported n-3 skew policy

**Remediation:**

```
Replace this node (managed node group rolling update, Karpenter Drift, or manual AMI bump) to pick up a kubelet within the supported skew window before proceeding with the next control-plane minor version upgrade. Deferred control-plane upgrades compound this: each skipped bump narrows the legal skew window for the next one.
```

### `PDB-001` PodDisruptionBudget demo/shared-app-pdb-b: disruptionsAllowed=0 (minAvailable: 1, currentHealthy=0, desiredHealthy=1, expectedPods=2) — matching pods cannot be voluntarily evicted, node drain will stall until the ~15-minute managed node group eviction budget expires

Confidence: `STATIC_CERTAIN`

**Evidence:**

- disruptionsAllowed: 0
- minAvailable: 1
- currentHealthy: 0
- desiredHealthy: 1
- expectedPods: 2

**Remediation:**

```
Safest-first remediation ladder: (1) scale up replicas to create eviction headroom without changing the PDB contract; (2) add topologySpreadConstraints to distribute the disruption cost across nodes; (3) temporarily relax this PDB for the change window, with an explicit revert step in the change ticket. Force-updating the node group to bypass PDBs is a last resort and must be a recorded business decision, not a default.
```

### `PDB-001` PodDisruptionBudget demo/singleton-app-pdb: disruptionsAllowed=0 (minAvailable: 1, currentHealthy=1, desiredHealthy=1, expectedPods=1) — matching pods cannot be voluntarily evicted, node drain will stall until the ~15-minute managed node group eviction budget expires

Confidence: `STATIC_CERTAIN`

**Evidence:**

- disruptionsAllowed: 0
- minAvailable: 1
- currentHealthy: 1
- desiredHealthy: 1
- expectedPods: 1

**Remediation:**

```
Safest-first remediation ladder: (1) scale up replicas to create eviction headroom without changing the PDB contract; (2) add topologySpreadConstraints to distribute the disruption cost across nodes; (3) temporarily relax this PDB for the change window, with an explicit revert step in the change ticket. Force-updating the node group to bypass PDBs is a last resort and must be a recorded business decision, not a default.
```

### `PDB-002` PodDisruptionBudgets demo/shared-app-pdb-a and demo/shared-app-pdb-b select an overlapping set of pods (2 overlapping: shared-app-5d96875494-8fxmd, shared-app-5d96875494-jcjdw) — the Eviction API rejects eviction when multiple PDBs match the same pod, even if each individually would allow disruption

Confidence: `STATIC_CERTAIN`

**Evidence:**

- PDB A: demo/shared-app-pdb-a (selector: app=shared-app)
- PDB B: demo/shared-app-pdb-b (selector: app=shared-app)
- overlapping pods: shared-app-5d96875494-8fxmd, shared-app-5d96875494-jcjdw

**Remediation:**

```
Overlap is always a misconfiguration: delete the duplicate/redundant PDB, or narrow one selector so the two budgets no longer target the same pods. If this is the AWS-managed CoreDNS PDB colliding with a hand-created duplicate in kube-system, delete the duplicate and keep the AWS-managed one.
```

### `WH-002` ValidatingWebhookConfiguration "demo-catchall-guard": webhook "guard.demo.kubepreflight.io" (index 0 in .webhooks) is fail-closed and its backend service demo/dead-guard-svc has zero ready endpoints — matching API writes will be rejected

Confidence: `STATIC_CERTAIN`

**Evidence:**

- webhook name: guard.demo.kubepreflight.io
- webhook index: 0
- backend service: demo/dead-guard-svc
- ready endpoint address count: 0

**Remediation:**

```
Narrow scope or fail-open temporarily, then restore backend health:

# Inventory
kubectl get validatingwebhookconfigurations,mutatingwebhookconfigurations -o wide

# Check backend health for this webhook's service
kubectl get endpointslices -n demo -l kubernetes.io/service-name=dead-guard-svc

# Mitigate (temporary): narrow scope or fail-open
kubectl patch validatingwebhookconfiguration demo-catchall-guard --type='json' \
  -p='[{"op":"replace","path":"/webhooks/0/failurePolicy","value":"Ignore"}]'

# Break-glass (cluster is bricked by the webhook): delete the config
kubectl delete validatingwebhookconfiguration demo-catchall-guard   # restore after recovery
```

## Warnings (2)

### `COREDNS-001` CoreDNS Corefile (kube-system/coredns) is missing the `ready` plugin — the CoreDNS pod's readiness probe can't reflect actual DNS server health, so a pod can be marked Ready before CoreDNS is actually serving, most likely to surface right after an add-on update

Confidence: `STATIC_CERTAIN`

**Evidence:**

- Corefile has no standalone `ready` directive

**Remediation:**

```
Add `ready` as a standalone directive inside the server block (typically alongside `health`). Back up the Corefile ConfigMap first, then apply the change directly with `kubectl apply` or via `aws eks update-addon --addon-name coredns --resolve-conflicts PRESERVE` if CoreDNS is managed as an EKS add-on.
```

### `WH-001` ValidatingWebhookConfiguration "demo-catchall-guard": webhook "guard.demo.kubepreflight.io" is fail-closed with catch-all scope (apiGroups: ["*"], resources: ["*"]) — every matching write in the cluster, including kube-system objects, depends on this webhook's backend being healthy

Confidence: `STATIC_CERTAIN`

**Evidence:**

- webhook name: guard.demo.kubepreflight.io
- scope: apiGroups=["*"], resources=["*"]
- failurePolicy: Fail (or unset, which defaults to Fail)

**Remediation:**

```
Narrow the webhook's rules to the specific apiGroups/resources it actually needs to validate/mutate, and add a namespaceSelector excluding kube-system and other critical namespaces. If this webhook does simple field validation, consider migrating it to a ValidatingAdmissionPolicy (CEL) to remove the callback dependency entirely.
```

## Next Actions (8)

1. **[Blocker] PodSecurityPolicy/demo-restricted** (API-001)

   ```
   Migrate to Pod Security Admission or a policy engine (Kyverno/Gatekeeper) before upgrading past 1.25. For manifests: `kubectl convert -f <file> --output-version <group>/<version>`. For Helm releases whose stored release manifest still references policy/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

2. **[Blocker] PodDisruptionBudget/demo/shared-app-pdb-b** (API-001, PDB-001)

   ```
   Migrate to policy/v1 PodDisruptionBudget before upgrading past 1.25. For manifests: `kubectl convert -f <file> --output-version <group>/<version>`. For Helm releases whose stored release manifest still references policy/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

   Also see `PDB-001`: Safest-first remediation ladder: (1) scale up replicas to create eviction headroom without changing the PDB contract; (2) add topologySpreadConstraints to distribute the disruption cost across nodes; (3) temporarily relax this PDB for the change window, with an explicit revert step in the change ticket. Force-updating the node group to bypass PDBs is a last resort and must be a recorded business decision, not a default.

3. **[Blocker] PodDisruptionBudget/demo/shared-app-pdb-a,shared-app-pdb-b** (PDB-002)

   ```
   Overlap is always a misconfiguration: delete the duplicate/redundant PDB, or narrow one selector so the two budgets no longer target the same pods. If this is the AWS-managed CoreDNS PDB colliding with a hand-created duplicate in kube-system, delete the duplicate and keep the AWS-managed one.
   ```

4. **[Blocker] PodDisruptionBudget/demo/singleton-app-pdb** (API-001, PDB-001)

   ```
   Migrate to policy/v1 PodDisruptionBudget before upgrading past 1.25. For manifests: `kubectl convert -f <file> --output-version <group>/<version>`. For Helm releases whose stored release manifest still references policy/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

   Also see `PDB-001`: Safest-first remediation ladder: (1) scale up replicas to create eviction headroom without changing the PDB contract; (2) add topologySpreadConstraints to distribute the disruption cost across nodes; (3) temporarily relax this PDB for the change window, with an explicit revert step in the change ticket. Force-updating the node group to bypass PDBs is a last resort and must be a recorded business decision, not a default.

5. **[Blocker] Node/kubepreflight-demo-control-plane** (NODE-001)

   ```
   Replace this node (managed node group rolling update, Karpenter Drift, or manual AMI bump) to pick up a kubelet within the supported skew window before proceeding with the next control-plane minor version upgrade. Deferred control-plane upgrades compound this: each skipped bump narrows the legal skew window for the next one.
   ```

6. **[Blocker] PodDisruptionBudget/demo/shared-app-pdb-a** (API-001)

   ```
   Migrate to policy/v1 PodDisruptionBudget before upgrading past 1.25. For manifests: `kubectl convert -f <file> --output-version <group>/<version>`. For Helm releases whose stored release manifest still references policy/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

7. **[Blocker] ValidatingWebhookConfiguration/demo-catchall-guard** (WH-001, WH-002)

   ```
   Narrow scope or fail-open temporarily, then restore backend health:
   
   # Inventory
   kubectl get validatingwebhookconfigurations,mutatingwebhookconfigurations -o wide
   
   # Check backend health for this webhook's service
   kubectl get endpointslices -n demo -l kubernetes.io/service-name=dead-guard-svc
   
   # Mitigate (temporary): narrow scope or fail-open
   kubectl patch validatingwebhookconfiguration demo-catchall-guard --type='json' \
     -p='[{"op":"replace","path":"/webhooks/0/failurePolicy","value":"Ignore"}]'
   
   # Break-glass (cluster is bricked by the webhook): delete the config
   kubectl delete validatingwebhookconfiguration demo-catchall-guard   # restore after recovery
   ```

   Also see `WH-001`: Narrow the webhook's rules to the specific apiGroups/resources it actually needs to validate/mutate, and add a namespaceSelector excluding kube-system and other critical namespaces. If this webhook does simple field validation, consider migrating it to a ValidatingAdmissionPolicy (CEL) to remove the callback dependency entirely.

8. **[Warning] ConfigMap/kube-system/coredns** (COREDNS-001)

   ```
   Add `ready` as a standalone directive inside the server block (typically alongside `health`). Back up the Corefile ConfigMap first, then apply the change directly with `kubectl apply` or via `aws eks update-addon --addon-name coredns --resolve-conflicts PRESERVE` if CoreDNS is managed as an EKS add-on.
   ```

## Evidence Appendix

Every finding's raw identity data, unmerged — cross-reference by fingerprint for waivers/dedup.

| Rule ID | Severity | Confidence | Resource | Fingerprint |
|---|---|---|---|---|
| API-001 | Blocker | STATIC_CERTAIN | PodDisruptionBudget/demo/shared-app-pdb-a | `36cb521e132bc4faeb8ded5709d52993da4ed27eac7574604f01bad5982fc04b` |
| API-001 | Blocker | STATIC_CERTAIN | PodDisruptionBudget/demo/shared-app-pdb-b | `06f186afc01178e8032232dfa3864060bc8525fcadfb2105089706d3d3bef74c` |
| API-001 | Blocker | STATIC_CERTAIN | PodDisruptionBudget/demo/singleton-app-pdb | `cb804097cb7f43763a1b413ae9502ab9d9de518bac7ecca9346979f3e9d1baab` |
| API-001 | Blocker | STATIC_CERTAIN | PodSecurityPolicy/demo-restricted | `10646905cb023d48e9918c262298e2c489572d7176d7b3eb2bbcb9ce366ebcbf` |
| COREDNS-001 | Warning | STATIC_CERTAIN | ConfigMap/kube-system/coredns | `a420e3934b41962784d0bf10fde52b351c1aeb655660df0cef7b669223fb9ba7` |
| NODE-001 | Blocker | STATIC_CERTAIN | Node/kubepreflight-demo-control-plane | `7aa0e34b95afe9aaa612f109f25a4503ba7994490892f7216e177a5452036412` |
| PDB-001 | Blocker | STATIC_CERTAIN | PodDisruptionBudget/demo/shared-app-pdb-b | `33f264ee826fd87a2a0ab55c30a657930705297a79be08315a0667a4b15209ff` |
| PDB-001 | Blocker | STATIC_CERTAIN | PodDisruptionBudget/demo/singleton-app-pdb | `d20021d57af2f301b8befe8102b85385c1edf52a4106dfeec9bc4c5402b5620f` |
| PDB-002 | Blocker | STATIC_CERTAIN | PodDisruptionBudget/demo/shared-app-pdb-a,shared-app-pdb-b | `aefe0d92b0b59172167dab47f123d29e7a83fa89065f964821fee77f4d079a6e` |
| WH-001 | Warning | STATIC_CERTAIN | ValidatingWebhookConfiguration/demo-catchall-guard | `b0d3e70e7b6d1a7ef56651ab578150fdc8935dbf332e1bcbc3dda69addc02721` |
| WH-002 | Blocker | STATIC_CERTAIN | ValidatingWebhookConfiguration/demo-catchall-guard | `d9168e90cde35088961fc18afe2951825607e6651f7b5e34896cdaab5144df1e` |

