# KubePreflight Scan Report

| | |
|---|---|
| **Cluster** | kind-kubepreflight-demo |
| **Target version** | 1.34 |
| **Provider** | cluster-only |
| **Scanned at** | 2026-07-03 18:34:32 UTC |
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
Migrate to policy/v1 PodDisruptionBudget before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references policy/v1beta1, migrate the stored release manifest too. If a controller/operator writes this object, upgrade that controller.
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
Migrate to policy/v1 PodDisruptionBudget before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references policy/v1beta1, migrate the stored release manifest too. If a controller/operator writes this object, upgrade that controller.
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
Migrate to policy/v1 PodDisruptionBudget before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references policy/v1beta1, migrate the stored release manifest too. If a controller/operator writes this object, upgrade that controller.
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
Migrate to Pod Security Admission or a policy engine (Kyverno/Gatekeeper) before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references policy/v1beta1, migrate the stored release manifest too. If a controller/operator writes this object, upgrade that controller.
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

### `PDB-001` PodDisruptionBudget demo/shared-app-pdb-b: disruptionsAllowed=0 (minAvailable: 1, currentHealthy=0, desiredHealthy=1, expectedPods=2) — healthy matching pods cannot currently be voluntarily evicted, so a node drain or node upgrade can stall or fail

Confidence: `OBSERVED`

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

### `PDB-001` PodDisruptionBudget demo/singleton-app-pdb: disruptionsAllowed=0 (minAvailable: 1, currentHealthy=1, desiredHealthy=1, expectedPods=1) — healthy matching pods cannot currently be voluntarily evicted, so a node drain or node upgrade can stall or fail

Confidence: `OBSERVED`

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

### `PDB-002` PodDisruptionBudgets demo/shared-app-pdb-a and demo/shared-app-pdb-b select an overlapping set of pods (2 overlapping: shared-app-5d96875494-dcpmr, shared-app-5d96875494-q6mbs) — the Eviction API rejects eviction when multiple PDBs match the same pod, even if each individually would allow disruption

Confidence: `OBSERVED`

**Evidence:**

- PDB A: demo/shared-app-pdb-a (selector: app=shared-app)
- PDB B: demo/shared-app-pdb-b (selector: app=shared-app)
- overlapping pods: shared-app-5d96875494-dcpmr, shared-app-5d96875494-q6mbs

**Remediation:**

```
Inspect both PDBs and their owners first. Then delete only a budget confirmed to be duplicate/redundant, or narrow one selector so each pod is selected by at most one PDB. For an AWS-managed CoreDNS PDB collision, confirm ownership before retaining the managed budget and removing the duplicate.
```

### `WH-002` ValidatingWebhookConfiguration "demo-catchall-guard": webhook "guard.demo.kubepreflight.io" (index 0 in .webhooks) is fail-closed and its backend service demo/dead-guard-svc has zero ready endpoints — matching API writes will be rejected

Confidence: `OBSERVED`

**Evidence:**

- webhook name: guard.demo.kubepreflight.io
- webhook index: 0
- backend service: demo/dead-guard-svc
- ready endpoint address count: 0

**Remediation:**

```
Restore the webhook backend, then verify ready endpoints:

kubectl get svc dead-guard-svc -n demo
kubectl get endpointslices -n demo -l kubernetes.io/service-name=dead-guard-svc
kubectl get deploy,pods -n demo

# Temporary incident mitigation only; the test operation guards the array index
kubectl patch validatingwebhookconfiguration demo-catchall-guard --type='json' \
  -p='[{"op":"test","path":"/webhooks/0/name","value":"guard.demo.kubepreflight.io"},{"op":"replace","path":"/webhooks/0/failurePolicy","value":"Ignore"}]'

Revert failurePolicy to Fail immediately after the backend recovers.
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

### `WH-001` ValidatingWebhookConfiguration "demo-catchall-guard": webhook "guard.demo.kubepreflight.io" is fail-closed with catch-all resource rules (apiGroups: ["*"], resources: ["*"], operations: [CREATE, UPDATE]) — all namespaces and cluster-scoped resources depend on this webhook's backend being healthy

Confidence: `STATIC_CERTAIN`

**Evidence:**

- webhook name: guard.demo.kubepreflight.io
- scope: apiGroups=["*"], resources=["*"]
- failurePolicy: Fail (or unset, which defaults to Fail)

**Remediation:**

```
Narrow the webhook's rules to the specific apiGroups/resources it actually needs to validate/mutate, and add a namespaceSelector excluding kube-system and other critical namespaces. If this webhook does simple field validation, consider migrating it to a ValidatingAdmissionPolicy (CEL) to remove the callback dependency entirely.
```

## Next Actions (6)

1. **[Blocker] PodSecurityPolicy/demo-restricted** (API-001)

   ```
   Migrate to Pod Security Admission or a policy engine (Kyverno/Gatekeeper) before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references policy/v1beta1, migrate the stored release manifest too. If a controller/operator writes this object, upgrade that controller.
   ```

2. **[Blocker] PodDisruptionBudget/demo/shared-app-pdb-a** (API-001, API-001, PDB-001, PDB-002)

   ```
   Migrate to policy/v1 PodDisruptionBudget before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references policy/v1beta1, migrate the stored release manifest too. If a controller/operator writes this object, upgrade that controller.
   ```

   Also see `PDB-001`: Safest-first remediation ladder: (1) scale up replicas to create eviction headroom without changing the PDB contract; (2) add topologySpreadConstraints to distribute the disruption cost across nodes; (3) temporarily relax this PDB for the change window, with an explicit revert step in the change ticket. Force-updating the node group to bypass PDBs is a last resort and must be a recorded business decision, not a default.

   Also see `PDB-002`: Inspect both PDBs and their owners first. Then delete only a budget confirmed to be duplicate/redundant, or narrow one selector so each pod is selected by at most one PDB. For an AWS-managed CoreDNS PDB collision, confirm ownership before retaining the managed budget and removing the duplicate.

3. **[Blocker] PodDisruptionBudget/demo/singleton-app-pdb** (API-001, PDB-001)

   ```
   Migrate to policy/v1 PodDisruptionBudget before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references policy/v1beta1, migrate the stored release manifest too. If a controller/operator writes this object, upgrade that controller.
   ```

   Also see `PDB-001`: Safest-first remediation ladder: (1) scale up replicas to create eviction headroom without changing the PDB contract; (2) add topologySpreadConstraints to distribute the disruption cost across nodes; (3) temporarily relax this PDB for the change window, with an explicit revert step in the change ticket. Force-updating the node group to bypass PDBs is a last resort and must be a recorded business decision, not a default.

4. **[Blocker] Node/kubepreflight-demo-control-plane** (NODE-001)

   ```
   Replace this node (managed node group rolling update, Karpenter Drift, or manual AMI bump) to pick up a kubelet within the supported skew window before proceeding with the next control-plane minor version upgrade. Deferred control-plane upgrades compound this: each skipped bump narrows the legal skew window for the next one.
   ```

5. **[Blocker] ValidatingWebhookConfiguration/demo-catchall-guard** (WH-001, WH-002)

   ```
   Restore the webhook backend, then verify ready endpoints:
   
   kubectl get svc dead-guard-svc -n demo
   kubectl get endpointslices -n demo -l kubernetes.io/service-name=dead-guard-svc
   kubectl get deploy,pods -n demo
   
   # Temporary incident mitigation only; the test operation guards the array index
   kubectl patch validatingwebhookconfiguration demo-catchall-guard --type='json' \
     -p='[{"op":"test","path":"/webhooks/0/name","value":"guard.demo.kubepreflight.io"},{"op":"replace","path":"/webhooks/0/failurePolicy","value":"Ignore"}]'
   
   Revert failurePolicy to Fail immediately after the backend recovers.
   ```

   Also see `WH-001`: Narrow the webhook's rules to the specific apiGroups/resources it actually needs to validate/mutate, and add a namespaceSelector excluding kube-system and other critical namespaces. If this webhook does simple field validation, consider migrating it to a ValidatingAdmissionPolicy (CEL) to remove the callback dependency entirely.

6. **[Warning] ConfigMap/kube-system/coredns** (COREDNS-001)

   ```
   Add `ready` as a standalone directive inside the server block (typically alongside `health`). Back up the Corefile ConfigMap first, then apply the change directly with `kubectl apply` or via `aws eks update-addon --addon-name coredns --resolve-conflicts PRESERVE` if CoreDNS is managed as an EKS add-on.
   ```

## Evidence Appendix

Every finding's resource identity and fingerprint — cross-reference by fingerprint for waivers/dedup.

| Rule ID | Severity | Confidence | Resource | Fingerprint |
|---|---|---|---|---|
| API-001 | Blocker | STATIC_CERTAIN | PodDisruptionBudget/demo/shared-app-pdb-a | `36cb521e132bc4faeb8ded5709d52993da4ed27eac7574604f01bad5982fc04b` |
| API-001 | Blocker | STATIC_CERTAIN | PodDisruptionBudget/demo/shared-app-pdb-b | `06f186afc01178e8032232dfa3864060bc8525fcadfb2105089706d3d3bef74c` |
| API-001 | Blocker | STATIC_CERTAIN | PodDisruptionBudget/demo/singleton-app-pdb | `cb804097cb7f43763a1b413ae9502ab9d9de518bac7ecca9346979f3e9d1baab` |
| API-001 | Blocker | STATIC_CERTAIN | PodSecurityPolicy/demo-restricted | `10646905cb023d48e9918c262298e2c489572d7176d7b3eb2bbcb9ce366ebcbf` |
| COREDNS-001 | Warning | STATIC_CERTAIN | ConfigMap/kube-system/coredns | `a420e3934b41962784d0bf10fde52b351c1aeb655660df0cef7b669223fb9ba7` |
| NODE-001 | Blocker | STATIC_CERTAIN | Node/kubepreflight-demo-control-plane | `7aa0e34b95afe9aaa612f109f25a4503ba7994490892f7216e177a5452036412` |
| PDB-001 | Blocker | OBSERVED | PodDisruptionBudget/demo/shared-app-pdb-b | `33f264ee826fd87a2a0ab55c30a657930705297a79be08315a0667a4b15209ff` |
| PDB-001 | Blocker | OBSERVED | PodDisruptionBudget/demo/singleton-app-pdb | `d20021d57af2f301b8befe8102b85385c1edf52a4106dfeec9bc4c5402b5620f` |
| PDB-002 | Blocker | OBSERVED | PodDisruptionBudget/demo/shared-app-pdb-a,shared-app-pdb-b | `aefe0d92b0b59172167dab47f123d29e7a83fa89065f964821fee77f4d079a6e` |
| WH-001 | Warning | STATIC_CERTAIN | ValidatingWebhookConfiguration/demo-catchall-guard | `b0d3e70e7b6d1a7ef56651ab578150fdc8935dbf332e1bcbc3dda69addc02721` |
| WH-002 | Blocker | OBSERVED | ValidatingWebhookConfiguration/demo-catchall-guard | `d9168e90cde35088961fc18afe2951825607e6651f7b5e34896cdaab5144df1e` |

