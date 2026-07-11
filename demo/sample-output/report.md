# KubePreflight Scan Report

| | |
|---|---|
| **Cluster** | kind-kubepreflight-demo |
| **Target version** | 1.34 |
| **Provider** | cluster-only |
| **Scanned at** | 2026-07-11 02:02:00 UTC |
| **Result** | **BLOCKED** |
| **Summary** | 98 blocker(s), 3 warning(s), 0 info(s) |

## Blockers (98)

### `P2` `API-001` EndpointSlice "default/kubernetes" (apiVersion discovery.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: discovery.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to discovery.k8s.io/v1 EndpointSlice before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references discovery.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` EndpointSlice "demo/dead-guard-svc-qrd2w" (apiVersion discovery.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: discovery.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to discovery.k8s.io/v1 EndpointSlice before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references discovery.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` EndpointSlice "kube-system/kube-dns-zj8ct" (apiVersion discovery.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: discovery.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to discovery.k8s.io/v1 EndpointSlice before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references discovery.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "default/kubepreflight-demo-control-plane.18c119c76c8eafa8" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "default/kubepreflight-demo-control-plane.18c119c770ee046d" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "default/kubepreflight-demo-control-plane.18c119c772b49864" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "default/kubepreflight-demo-control-plane.18c119c772b4af6d" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "default/kubepreflight-demo-control-plane.18c119c772b4bc0e" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "default/kubepreflight-demo-control-plane.18c119ca5d558648" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "default/kubepreflight-demo-control-plane.18c119cab1f091a9" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "default/kubepreflight-demo-control-plane.18c119cc33fc383d" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "demo/shared-app-5d96875494-9lh5m.18c119cf0229f07a" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "demo/shared-app-5d96875494-9lh5m.18c119cf022b20b3" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "demo/shared-app-5d96875494-9lh5m.18c119cf1be3f882" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "demo/shared-app-5d96875494-9lh5m.18c119cf9043fdf1" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "demo/shared-app-5d96875494-9lh5m.18c119cf91195da6" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "demo/shared-app-5d96875494-9lh5m.18c119cf9869caad" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "demo/shared-app-5d96875494-xnwsk.18c119cf01ce642e" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "demo/shared-app-5d96875494-xnwsk.18c119cf02c32754" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "demo/shared-app-5d96875494-xnwsk.18c119cf1b93d641" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "demo/shared-app-5d96875494-xnwsk.18c119cf7e1cd35e" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "demo/shared-app-5d96875494-xnwsk.18c119cf7ece15a3" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "demo/shared-app-5d96875494-xnwsk.18c119cf86da0b40" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "demo/shared-app-5d96875494.18c119cf0193feae" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "demo/shared-app-5d96875494.18c119cf01d77994" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "demo/shared-app-pdb-a.18c119cf01801233" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "demo/shared-app.18c119cf01564e8c" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "demo/singleton-app-685c8555c8-8hvnd.18c119cef6f9b5ad" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "demo/singleton-app-685c8555c8-8hvnd.18c119cf140ac89d" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "demo/singleton-app-685c8555c8-8hvnd.18c119cf697d141c" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "demo/singleton-app-685c8555c8-8hvnd.18c119cf6a4172c1" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "demo/singleton-app-685c8555c8-8hvnd.18c119cf7154a94b" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "demo/singleton-app-685c8555c8.18c119cef6b308d4" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "demo/singleton-app-pdb.18c119cef691ea71" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "demo/singleton-app.18c119cef6415cc4" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "kube-system/coredns-57575c5f89-d2hrs.18c119ca944db7c1" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "kube-system/coredns-57575c5f89-d2hrs.18c119cc3bfbdf6f" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "kube-system/coredns-57575c5f89-d2hrs.18c119cc66271e96" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "kube-system/coredns-57575c5f89-d2hrs.18c119cc6698a27d" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "kube-system/coredns-57575c5f89-d2hrs.18c119cc6d22c56a" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "kube-system/coredns-57575c5f89-d6dln.18c119ca938ddd34" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "kube-system/coredns-57575c5f89-d6dln.18c119cc3bf4a2a0" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "kube-system/coredns-57575c5f89-d6dln.18c119cc5824cb45" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "kube-system/coredns-57575c5f89-d6dln.18c119cc6369acf7" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "kube-system/coredns-57575c5f89-d6dln.18c119cc686608b2" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "kube-system/coredns-57575c5f89.18c119ca938b182c" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "kube-system/coredns-57575c5f89.18c119ca93df13e6" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "kube-system/coredns.18c119ca60d30d59" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "kube-system/kindnet-cn6z9.18c119ca7ee82343" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "kube-system/kindnet-cn6z9.18c119caa9198485" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "kube-system/kindnet-cn6z9.18c119cac5533665" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "kube-system/kindnet-cn6z9.18c119cafdcc084f" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "kube-system/kindnet.18c119ca7ea12743" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "kube-system/kube-controller-manager.18c119c7584c6a46" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "kube-system/kube-proxy-bcnz7.18c119ca7ee61dca" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "kube-system/kube-proxy-bcnz7.18c119ca98cf6471" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "kube-system/kube-proxy-bcnz7.18c119caa9374311" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "kube-system/kube-proxy-bcnz7.18c119caaee6ed7b" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "kube-system/kube-proxy.18c119ca7e978209" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "kube-system/kube-scheduler.18c119c79d11cb6f" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "local-path-storage/local-path-provisioner-c49b7b56f-66nwj.18c119ca94ef7ad0" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "local-path-storage/local-path-provisioner-c49b7b56f-66nwj.18c119cc3bfc3d7e" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "local-path-storage/local-path-provisioner-c49b7b56f-66nwj.18c119cc58501ecd" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "local-path-storage/local-path-provisioner-c49b7b56f-66nwj.18c119cc67b63df1" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "local-path-storage/local-path-provisioner-c49b7b56f-66nwj.18c119cc6d5f0d3c" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "local-path-storage/local-path-provisioner-c49b7b56f.18c119ca93a732bd" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` Event "local-path-storage/local-path-provisioner.18c119ca60d2bb69" (apiVersion events.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: events.k8s.io/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` FlowSchema "catch-all" (apiVersion flowcontrol.apiserver.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.26 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta1
- removed in: Kubernetes 1.26
- target version: 1.34
- detected via: live cluster object
- apiVersion: flowcontrol.apiserver.k8s.io/v1beta2
- removed in: Kubernetes 1.29

**Remediation:**

```
Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` FlowSchema "endpoint-controller" (apiVersion flowcontrol.apiserver.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.26 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta1
- removed in: Kubernetes 1.26
- target version: 1.34
- detected via: live cluster object
- apiVersion: flowcontrol.apiserver.k8s.io/v1beta2
- removed in: Kubernetes 1.29

**Remediation:**

```
Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` FlowSchema "exempt" (apiVersion flowcontrol.apiserver.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.26 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta1
- removed in: Kubernetes 1.26
- target version: 1.34
- detected via: live cluster object
- apiVersion: flowcontrol.apiserver.k8s.io/v1beta2
- removed in: Kubernetes 1.29

**Remediation:**

```
Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` FlowSchema "global-default" (apiVersion flowcontrol.apiserver.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.26 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta1
- removed in: Kubernetes 1.26
- target version: 1.34
- detected via: live cluster object
- apiVersion: flowcontrol.apiserver.k8s.io/v1beta2
- removed in: Kubernetes 1.29

**Remediation:**

```
Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` FlowSchema "kube-controller-manager" (apiVersion flowcontrol.apiserver.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.26 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta1
- removed in: Kubernetes 1.26
- target version: 1.34
- detected via: live cluster object
- apiVersion: flowcontrol.apiserver.k8s.io/v1beta2
- removed in: Kubernetes 1.29

**Remediation:**

```
Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` FlowSchema "kube-scheduler" (apiVersion flowcontrol.apiserver.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.26 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta1
- removed in: Kubernetes 1.26
- target version: 1.34
- detected via: live cluster object
- apiVersion: flowcontrol.apiserver.k8s.io/v1beta2
- removed in: Kubernetes 1.29

**Remediation:**

```
Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` FlowSchema "kube-system-service-accounts" (apiVersion flowcontrol.apiserver.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.26 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta1
- removed in: Kubernetes 1.26
- target version: 1.34
- detected via: live cluster object
- apiVersion: flowcontrol.apiserver.k8s.io/v1beta2
- removed in: Kubernetes 1.29

**Remediation:**

```
Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` FlowSchema "probes" (apiVersion flowcontrol.apiserver.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.26 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta1
- removed in: Kubernetes 1.26
- target version: 1.34
- detected via: live cluster object
- apiVersion: flowcontrol.apiserver.k8s.io/v1beta2
- removed in: Kubernetes 1.29

**Remediation:**

```
Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` FlowSchema "service-accounts" (apiVersion flowcontrol.apiserver.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.26 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta1
- removed in: Kubernetes 1.26
- target version: 1.34
- detected via: live cluster object
- apiVersion: flowcontrol.apiserver.k8s.io/v1beta2
- removed in: Kubernetes 1.29

**Remediation:**

```
Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` FlowSchema "system-leader-election" (apiVersion flowcontrol.apiserver.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.26 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta1
- removed in: Kubernetes 1.26
- target version: 1.34
- detected via: live cluster object
- apiVersion: flowcontrol.apiserver.k8s.io/v1beta2
- removed in: Kubernetes 1.29

**Remediation:**

```
Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` FlowSchema "system-node-high" (apiVersion flowcontrol.apiserver.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.26 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta1
- removed in: Kubernetes 1.26
- target version: 1.34
- detected via: live cluster object
- apiVersion: flowcontrol.apiserver.k8s.io/v1beta2
- removed in: Kubernetes 1.29

**Remediation:**

```
Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` FlowSchema "system-nodes" (apiVersion flowcontrol.apiserver.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.26 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta1
- removed in: Kubernetes 1.26
- target version: 1.34
- detected via: live cluster object
- apiVersion: flowcontrol.apiserver.k8s.io/v1beta2
- removed in: Kubernetes 1.29

**Remediation:**

```
Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` FlowSchema "workload-leader-election" (apiVersion flowcontrol.apiserver.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.26 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta1
- removed in: Kubernetes 1.26
- target version: 1.34
- detected via: live cluster object
- apiVersion: flowcontrol.apiserver.k8s.io/v1beta2
- removed in: Kubernetes 1.29

**Remediation:**

```
Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` PodDisruptionBudget "demo/shared-app-pdb-a" (apiVersion policy/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: policy/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to policy/v1 PodDisruptionBudget before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references policy/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` PodDisruptionBudget "demo/shared-app-pdb-b" (apiVersion policy/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: policy/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to policy/v1 PodDisruptionBudget before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references policy/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` PodDisruptionBudget "demo/singleton-app-pdb" (apiVersion policy/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: policy/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to policy/v1 PodDisruptionBudget before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references policy/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` PodSecurityPolicy "demo-restricted" (apiVersion policy/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: policy/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.34
- detected via: live cluster object

**Remediation:**

```
Migrate to Pod Security Admission or a policy engine (Kyverno/Gatekeeper) before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references policy/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` PriorityLevelConfiguration "catch-all" (apiVersion flowcontrol.apiserver.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.26 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta1
- removed in: Kubernetes 1.26
- target version: 1.34
- detected via: live cluster object
- apiVersion: flowcontrol.apiserver.k8s.io/v1beta2
- removed in: Kubernetes 1.29

**Remediation:**

```
Migrate to flowcontrol.apiserver.k8s.io/v1 PriorityLevelConfiguration before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` PriorityLevelConfiguration "exempt" (apiVersion flowcontrol.apiserver.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.26 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta1
- removed in: Kubernetes 1.26
- target version: 1.34
- detected via: live cluster object
- apiVersion: flowcontrol.apiserver.k8s.io/v1beta2
- removed in: Kubernetes 1.29

**Remediation:**

```
Migrate to flowcontrol.apiserver.k8s.io/v1 PriorityLevelConfiguration before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` PriorityLevelConfiguration "global-default" (apiVersion flowcontrol.apiserver.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.26 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta1
- removed in: Kubernetes 1.26
- target version: 1.34
- detected via: live cluster object
- apiVersion: flowcontrol.apiserver.k8s.io/v1beta2
- removed in: Kubernetes 1.29

**Remediation:**

```
Migrate to flowcontrol.apiserver.k8s.io/v1 PriorityLevelConfiguration before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` PriorityLevelConfiguration "leader-election" (apiVersion flowcontrol.apiserver.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.26 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta1
- removed in: Kubernetes 1.26
- target version: 1.34
- detected via: live cluster object
- apiVersion: flowcontrol.apiserver.k8s.io/v1beta2
- removed in: Kubernetes 1.29

**Remediation:**

```
Migrate to flowcontrol.apiserver.k8s.io/v1 PriorityLevelConfiguration before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` PriorityLevelConfiguration "node-high" (apiVersion flowcontrol.apiserver.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.26 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta1
- removed in: Kubernetes 1.26
- target version: 1.34
- detected via: live cluster object
- apiVersion: flowcontrol.apiserver.k8s.io/v1beta2
- removed in: Kubernetes 1.29

**Remediation:**

```
Migrate to flowcontrol.apiserver.k8s.io/v1 PriorityLevelConfiguration before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` PriorityLevelConfiguration "system" (apiVersion flowcontrol.apiserver.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.26 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta1
- removed in: Kubernetes 1.26
- target version: 1.34
- detected via: live cluster object
- apiVersion: flowcontrol.apiserver.k8s.io/v1beta2
- removed in: Kubernetes 1.29

**Remediation:**

```
Migrate to flowcontrol.apiserver.k8s.io/v1 PriorityLevelConfiguration before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` PriorityLevelConfiguration "workload-high" (apiVersion flowcontrol.apiserver.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.26 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta1
- removed in: Kubernetes 1.26
- target version: 1.34
- detected via: live cluster object
- apiVersion: flowcontrol.apiserver.k8s.io/v1beta2
- removed in: Kubernetes 1.29

**Remediation:**

```
Migrate to flowcontrol.apiserver.k8s.io/v1 PriorityLevelConfiguration before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `API-001` PriorityLevelConfiguration "workload-low" (apiVersion flowcontrol.apiserver.k8s.io/v1beta1) still exists at a version removed in Kubernetes 1.26 — target version 1.34 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: flowcontrol.apiserver.k8s.io/v1beta1
- removed in: Kubernetes 1.26
- target version: 1.34
- detected via: live cluster object
- apiVersion: flowcontrol.apiserver.k8s.io/v1beta2
- removed in: Kubernetes 1.29

**Remediation:**

```
Migrate to flowcontrol.apiserver.k8s.io/v1 PriorityLevelConfiguration before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
```

### `P2` `NODE-003` Critical component Deployment kube-system/coredns depends on the deprecated node-role.kubernetes.io/master node label — it may fail to schedule after a control-plane node rebuild or upgrade, taking cluster infrastructure down with it

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- references node-role.kubernetes.io/master at spec.template.spec.tolerations[1].key
- replacement label: node-role.kubernetes.io/control-plane (kubeadm stopped adding the master label to new control-plane nodes in Kubernetes 1.24)

**Remediation:**

```
Replace deprecated node-role.kubernetes.io/master references with node-role.kubernetes.io/control-plane, or migrate to an explicit stable node label managed by the platform team. Validate that all target nodes already carry the replacement label before changing selectors or affinities — changing the selector first strands the workload with no schedulable nodes.
```

### `P3` `NODE-001` Node "kubepreflight-demo-control-plane": kubelet version v1.24.15 is outside the supported skew window for target version 1.34 — kubelet minor version 24 is 10 minor versions behind target minor version 34 — exceeds the supported n-3 skew policy

Confidence: `STATIC_CERTAIN` · Can upgrade continue: No

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- kubelet version: v1.24.15
- target version: 1.34
- kubelet minor version 24 is 10 minor versions behind target minor version 34 — exceeds the supported n-3 skew policy

**Remediation:**

```
Replace this node (managed node group rolling update, Karpenter Drift, or manual AMI bump) to pick up a kubelet within the supported skew window before proceeding with the next control-plane minor version upgrade. Deferred control-plane upgrades compound this: each skipped bump narrows the legal skew window for the next one.
```

### `P3` `PDB-001` PodDisruptionBudget demo/shared-app-pdb-b: disruptionsAllowed=0 (minAvailable: 1, currentHealthy=0, desiredHealthy=1, expectedPods=2) — healthy matching pods cannot currently be voluntarily evicted, so a node drain or node upgrade can stall or fail

Confidence: `OBSERVED` · Can upgrade continue: No

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- disruptionsAllowed: 0
- minAvailable: 1
- currentHealthy: 0
- desiredHealthy: 1
- expectedPods: 2
- observedGeneration: 1 (metadata.generation: 1)

**Remediation:**

```
Safest-first remediation ladder: (1) scale up replicas to create eviction headroom without changing the PDB contract; (2) add topologySpreadConstraints to distribute the disruption cost across nodes; (3) temporarily relax this PDB for the change window, with an explicit revert step in the change ticket. Force-updating the node group to bypass PDBs is a last resort and must be a recorded business decision, not a default.
```

### `P3` `PDB-001` PodDisruptionBudget demo/singleton-app-pdb: disruptionsAllowed=0 (minAvailable: 1, currentHealthy=1, desiredHealthy=1, expectedPods=1) — healthy matching pods cannot currently be voluntarily evicted, so a node drain or node upgrade can stall or fail

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

### `P3` `PDB-002` PodDisruptionBudgets demo/shared-app-pdb-a and demo/shared-app-pdb-b select an overlapping set of pods (2 overlapping: shared-app-5d96875494-9lh5m, shared-app-5d96875494-xnwsk) — the Eviction API rejects eviction when multiple PDBs match the same pod, even if each individually would allow disruption

Confidence: `OBSERVED` · Can upgrade continue: No

> **Why this matters (P3):** Node drain may fail during maintenance or a managed node group upgrade.

**Evidence:**

- PDB A: demo/shared-app-pdb-a (selector: app=shared-app)
- PDB B: demo/shared-app-pdb-b (selector: app=shared-app)
- overlapping pods: shared-app-5d96875494-9lh5m, shared-app-5d96875494-xnwsk

**Remediation:**

```
Inspect both PDBs and their owners first. Then delete only a budget confirmed to be duplicate/redundant, or narrow one selector so each pod is selected by at most one PDB. For an AWS-managed CoreDNS PDB collision, confirm ownership before retaining the managed budget and removing the duplicate.
```

### `P4` `WH-002` ValidatingWebhookConfiguration "demo-catchall-guard": webhook "guard.demo.kubepreflight.io" (index 0 in .webhooks) is fail-closed and its backend service demo/dead-guard-svc has zero ready endpoints — matching API writes will be rejected

Confidence: `OBSERVED` · Can upgrade continue: No

> **Why this matters (P4):** Upgrade should not begin while workloads, nodes, or critical add-ons are unhealthy.

**Evidence:**

- webhook name: guard.demo.kubepreflight.io
- webhook index: 0
- backend service: demo/dead-guard-svc
- ready endpoint address count: 0
- failurePolicy: Fail

**Remediation:**

```
Step 1 — restore the webhook backend (read-only, safe to run any time):

kubectl get svc 'dead-guard-svc' -n 'demo'
kubectl get endpointslices -n 'demo' -l kubernetes.io/service-name='dead-guard-svc'
kubectl get deploy,pods -n 'demo'

Step 2 — only if you need immediate relief and cannot wait for the backend to recover:

This TEMPORARILY REMOVES the webhook's protection. The "test" operation
guards against the array index having shifted since this scan ran — the
patch aborts instead of silently touching the wrong webhook block.

kubectl patch validatingwebhookconfiguration 'demo-catchall-guard' --type='json' -p='[{"op":"test","path":"/webhooks/0/name","value":"guard.demo.kubepreflight.io"},{"op":"replace","path":"/webhooks/0/failurePolicy","value":"Ignore"}]'

Revert failurePolicy to Fail immediately after the backend recovers.
```

## Warnings (3)

### `P4` `COREDNS-001` CoreDNS Corefile (kube-system/coredns) is missing the `ready` plugin — the CoreDNS pod's readiness probe can't reflect actual DNS server health, so a pod can be marked Ready before CoreDNS is actually serving, most likely to surface right after an add-on update

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P4):** Upgrade should not begin while workloads, nodes, or critical add-ons are unhealthy.

**Evidence:**

- Corefile has no standalone `ready` directive

**Remediation:**

```
Add `ready` as a standalone directive inside the server block (typically alongside `health`). Back up the Corefile ConfigMap first, then apply the change directly with `kubectl apply` or via `aws eks update-addon --addon-name coredns --resolve-conflicts PRESERVE` if CoreDNS is managed as an EKS add-on.
```

### `P4` `NODE-003` Deployment local-path-storage/local-path-provisioner schedules using the deprecated node-role.kubernetes.io/master node label — new control-plane nodes carry node-role.kubernetes.io/control-plane instead, so this workload may fail to schedule after a control-plane node rebuild, cluster replacement, or platform label cleanup

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P4):** Upgrade should not begin while workloads, nodes, or critical add-ons are unhealthy.

**Evidence:**

- references node-role.kubernetes.io/master at spec.template.spec.tolerations[1].key
- replacement label: node-role.kubernetes.io/control-plane (kubeadm stopped adding the master label to new control-plane nodes in Kubernetes 1.24)

**Remediation:**

```
Replace deprecated node-role.kubernetes.io/master references with node-role.kubernetes.io/control-plane, or migrate to an explicit stable node label managed by the platform team. Validate that all target nodes already carry the replacement label before changing selectors or affinities — changing the selector first strands the workload with no schedulable nodes.
```

### `P4` `WH-001` ValidatingWebhookConfiguration "demo-catchall-guard": webhook "guard.demo.kubepreflight.io" is fail-closed with catch-all resource rules (apiGroups: ["*"], resources: ["*"], operations: [CREATE,UPDATE]) — requests that also satisfy its configured selectors/match conditions depend on this webhook's backend being healthy

Confidence: `STATIC_CERTAIN` · Can upgrade continue: Yes

> **Why this matters (P4):** Upgrade should not begin while workloads, nodes, or critical add-ons are unhealthy.

**Evidence:**

- webhook name: guard.demo.kubepreflight.io
- scope: apiGroups=["*"], resources=["*"]
- operations: [CREATE,UPDATE]
- failurePolicy: Fail
- namespaceSelector set: true
- objectSelector set: false
- matchConditions set: false

**Remediation:**

```
Inspect the webhook's current rules and selectors:

kubectl get validatingwebhookconfiguration 'demo-catchall-guard' -o yaml

Then narrow the webhook's rules to the specific apiGroups/resources it actually needs to
validate/mutate, and add a namespaceSelector excluding kube-system and other critical
namespaces. If this webhook does simple field validation, consider migrating it to a
ValidatingAdmissionPolicy (CEL) to remove the callback dependency entirely.
```

## Next Actions (96)

1. **[P2/Blocker] Event/demo/shared-app-5d96875494-9lh5m.18c119cf1be3f882** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

2. **[P2/Blocker] Event/kube-system/coredns-57575c5f89.18c119ca93df13e6** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

3. **[P2/Blocker] Event/kube-system/kube-proxy-bcnz7.18c119caaee6ed7b** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

4. **[P2/Blocker] PodSecurityPolicy/demo-restricted** (API-001)

   ```
   Migrate to Pod Security Admission or a policy engine (Kyverno/Gatekeeper) before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references policy/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

5. **[P2/Blocker] Event/kube-system/kindnet-cn6z9.18c119cac5533665** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

6. **[P2/Blocker] FlowSchema/system-nodes** (API-001)

   ```
   Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

7. **[P2/Blocker] FlowSchema/kube-scheduler** (API-001)

   ```
   Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

8. **[P2/Blocker] FlowSchema/kube-system-service-accounts** (API-001)

   ```
   Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

9. **[P2/Blocker] Event/demo/singleton-app-685c8555c8-8hvnd.18c119cf6a4172c1** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

10. **[P2/Blocker] Event/kube-system/coredns-57575c5f89-d2hrs.18c119ca944db7c1** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

11. **[P2/Blocker] Event/local-path-storage/local-path-provisioner-c49b7b56f-66nwj.18c119cc6d5f0d3c** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

12. **[P2/Blocker] Event/kube-system/coredns-57575c5f89-d6dln.18c119cc3bf4a2a0** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

13. **[P2/Blocker] Event/kube-system/coredns.18c119ca60d30d59** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

14. **[P2/Blocker] FlowSchema/catch-all** (API-001)

   ```
   Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

15. **[P2/Blocker] FlowSchema/kube-controller-manager** (API-001)

   ```
   Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

16. **[P2/Blocker] Event/demo/shared-app-5d96875494-xnwsk.18c119cf86da0b40** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

17. **[P2/Blocker] PodDisruptionBudget/demo/shared-app-pdb-a** (API-001, API-001, PDB-001, PDB-002)

   ```
   Migrate to policy/v1 PodDisruptionBudget before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references policy/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

   Also see `PDB-001`: Safest-first remediation ladder: (1) scale up replicas to create eviction headroom without changing the PDB contract; (2) add topologySpreadConstraints to distribute the disruption cost across nodes; (3) temporarily relax this PDB for the change window, with an explicit revert step in the change ticket. Force-updating the node group to bypass PDBs is a last resort and must be a recorded business decision, not a default.

   Also see `PDB-002`: Inspect both PDBs and their owners first. Then delete only a budget confirmed to be duplicate/redundant, or narrow one selector so each pod is selected by at most one PDB. For an AWS-managed CoreDNS PDB collision, confirm ownership before retaining the managed budget and removing the duplicate.

18. **[P2/Blocker] Event/kube-system/kindnet-cn6z9.18c119caa9198485** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

19. **[P2/Blocker] Event/kube-system/kube-scheduler.18c119c79d11cb6f** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

20. **[P2/Blocker] Event/demo/shared-app-5d96875494-xnwsk.18c119cf7e1cd35e** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

21. **[P2/Blocker] PodDisruptionBudget/demo/singleton-app-pdb** (API-001, PDB-001)

   ```
   Migrate to policy/v1 PodDisruptionBudget before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references policy/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

   Also see `PDB-001`: Safest-first remediation ladder: (1) scale up replicas to create eviction headroom without changing the PDB contract; (2) add topologySpreadConstraints to distribute the disruption cost across nodes; (3) temporarily relax this PDB for the change window, with an explicit revert step in the change ticket. Force-updating the node group to bypass PDBs is a last resort and must be a recorded business decision, not a default.

22. **[P2/Blocker] PriorityLevelConfiguration/catch-all** (API-001)

   ```
   Migrate to flowcontrol.apiserver.k8s.io/v1 PriorityLevelConfiguration before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

23. **[P2/Blocker] Event/local-path-storage/local-path-provisioner-c49b7b56f-66nwj.18c119ca94ef7ad0** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

24. **[P2/Blocker] Event/default/kubepreflight-demo-control-plane.18c119c770ee046d** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

25. **[P2/Blocker] PriorityLevelConfiguration/system** (API-001)

   ```
   Migrate to flowcontrol.apiserver.k8s.io/v1 PriorityLevelConfiguration before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

26. **[P2/Blocker] Event/kube-system/kube-controller-manager.18c119c7584c6a46** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

27. **[P2/Blocker] Deployment/kube-system/coredns** (NODE-003)

   ```
   Replace deprecated node-role.kubernetes.io/master references with node-role.kubernetes.io/control-plane, or migrate to an explicit stable node label managed by the platform team. Validate that all target nodes already carry the replacement label before changing selectors or affinities — changing the selector first strands the workload with no schedulable nodes.
   ```

28. **[P2/Blocker] Event/demo/shared-app-5d96875494-9lh5m.18c119cf0229f07a** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

29. **[P2/Blocker] Event/local-path-storage/local-path-provisioner.18c119ca60d2bb69** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

30. **[P2/Blocker] Event/demo/singleton-app-685c8555c8.18c119cef6b308d4** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

31. **[P2/Blocker] Event/kube-system/coredns-57575c5f89-d2hrs.18c119cc3bfbdf6f** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

32. **[P2/Blocker] Event/local-path-storage/local-path-provisioner-c49b7b56f-66nwj.18c119cc67b63df1** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

33. **[P2/Blocker] Event/default/kubepreflight-demo-control-plane.18c119ca5d558648** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

34. **[P2/Blocker] Event/local-path-storage/local-path-provisioner-c49b7b56f-66nwj.18c119cc58501ecd** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

35. **[P2/Blocker] Event/default/kubepreflight-demo-control-plane.18c119cab1f091a9** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

36. **[P2/Blocker] Event/kube-system/kube-proxy-bcnz7.18c119ca98cf6471** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

37. **[P2/Blocker] Event/demo/singleton-app-685c8555c8-8hvnd.18c119cf7154a94b** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

38. **[P2/Blocker] Event/kube-system/coredns-57575c5f89-d2hrs.18c119cc6d22c56a** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

39. **[P2/Blocker] Event/demo/shared-app-5d96875494-9lh5m.18c119cf022b20b3** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

40. **[P2/Blocker] PriorityLevelConfiguration/workload-high** (API-001)

   ```
   Migrate to flowcontrol.apiserver.k8s.io/v1 PriorityLevelConfiguration before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

41. **[P2/Blocker] Event/kube-system/kube-proxy-bcnz7.18c119caa9374311** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

42. **[P2/Blocker] FlowSchema/workload-leader-election** (API-001)

   ```
   Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

43. **[P2/Blocker] Event/kube-system/coredns-57575c5f89-d6dln.18c119ca938ddd34** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

44. **[P2/Blocker] EndpointSlice/default/kubernetes** (API-001)

   ```
   Migrate to discovery.k8s.io/v1 EndpointSlice before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references discovery.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

45. **[P2/Blocker] Event/demo/shared-app-5d96875494.18c119cf01d77994** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

46. **[P2/Blocker] PriorityLevelConfiguration/global-default** (API-001)

   ```
   Migrate to flowcontrol.apiserver.k8s.io/v1 PriorityLevelConfiguration before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

47. **[P2/Blocker] Event/kube-system/kindnet.18c119ca7ea12743** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

48. **[P2/Blocker] Event/demo/singleton-app-685c8555c8-8hvnd.18c119cef6f9b5ad** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

49. **[P2/Blocker] Event/demo/singleton-app-685c8555c8-8hvnd.18c119cf697d141c** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

50. **[P2/Blocker] Event/kube-system/kube-proxy-bcnz7.18c119ca7ee61dca** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

51. **[P2/Blocker] Event/kube-system/coredns-57575c5f89-d6dln.18c119cc5824cb45** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

52. **[P2/Blocker] Event/kube-system/kindnet-cn6z9.18c119cafdcc084f** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

53. **[P2/Blocker] PriorityLevelConfiguration/workload-low** (API-001)

   ```
   Migrate to flowcontrol.apiserver.k8s.io/v1 PriorityLevelConfiguration before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

54. **[P2/Blocker] Event/kube-system/coredns-57575c5f89-d6dln.18c119cc686608b2** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

55. **[P2/Blocker] Event/demo/shared-app-5d96875494-xnwsk.18c119cf01ce642e** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

56. **[P2/Blocker] FlowSchema/probes** (API-001)

   ```
   Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

57. **[P2/Blocker] Event/demo/shared-app-5d96875494-9lh5m.18c119cf91195da6** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

58. **[P2/Blocker] FlowSchema/global-default** (API-001)

   ```
   Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

59. **[P2/Blocker] Event/demo/shared-app-5d96875494-xnwsk.18c119cf1b93d641** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

60. **[P2/Blocker] Event/default/kubepreflight-demo-control-plane.18c119cc33fc383d** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

61. **[P2/Blocker] Event/kube-system/coredns-57575c5f89.18c119ca938b182c** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

62. **[P2/Blocker] FlowSchema/exempt** (API-001)

   ```
   Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

63. **[P2/Blocker] Event/demo/shared-app-5d96875494-xnwsk.18c119cf02c32754** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

64. **[P2/Blocker] FlowSchema/endpoint-controller** (API-001)

   ```
   Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

65. **[P2/Blocker] Event/demo/shared-app.18c119cf01564e8c** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

66. **[P2/Blocker] Event/demo/shared-app-5d96875494-xnwsk.18c119cf7ece15a3** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

67. **[P2/Blocker] Event/kube-system/coredns-57575c5f89-d6dln.18c119cc6369acf7** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

68. **[P2/Blocker] Event/demo/shared-app-5d96875494-9lh5m.18c119cf9869caad** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

69. **[P2/Blocker] PriorityLevelConfiguration/leader-election** (API-001)

   ```
   Migrate to flowcontrol.apiserver.k8s.io/v1 PriorityLevelConfiguration before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

70. **[P2/Blocker] EndpointSlice/kube-system/kube-dns-zj8ct** (API-001)

   ```
   Migrate to discovery.k8s.io/v1 EndpointSlice before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references discovery.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

71. **[P2/Blocker] Event/local-path-storage/local-path-provisioner-c49b7b56f-66nwj.18c119cc3bfc3d7e** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

72. **[P2/Blocker] Event/default/kubepreflight-demo-control-plane.18c119c772b49864** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

73. **[P2/Blocker] FlowSchema/service-accounts** (API-001)

   ```
   Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

74. **[P2/Blocker] Event/default/kubepreflight-demo-control-plane.18c119c772b4bc0e** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

75. **[P2/Blocker] PriorityLevelConfiguration/exempt** (API-001)

   ```
   Migrate to flowcontrol.apiserver.k8s.io/v1 PriorityLevelConfiguration before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

76. **[P2/Blocker] Event/demo/singleton-app.18c119cef6415cc4** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

77. **[P2/Blocker] Event/demo/shared-app-5d96875494.18c119cf0193feae** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

78. **[P2/Blocker] PriorityLevelConfiguration/node-high** (API-001)

   ```
   Migrate to flowcontrol.apiserver.k8s.io/v1 PriorityLevelConfiguration before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

79. **[P2/Blocker] Event/demo/shared-app-5d96875494-9lh5m.18c119cf9043fdf1** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

80. **[P2/Blocker] Event/kube-system/kindnet-cn6z9.18c119ca7ee82343** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

81. **[P2/Blocker] Event/default/kubepreflight-demo-control-plane.18c119c772b4af6d** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

82. **[P2/Blocker] EndpointSlice/demo/dead-guard-svc-qrd2w** (API-001)

   ```
   Migrate to discovery.k8s.io/v1 EndpointSlice before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references discovery.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

83. **[P2/Blocker] Event/local-path-storage/local-path-provisioner-c49b7b56f.18c119ca93a732bd** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

84. **[P2/Blocker] Event/kube-system/coredns-57575c5f89-d2hrs.18c119cc6698a27d** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

85. **[P2/Blocker] Event/kube-system/kube-proxy.18c119ca7e978209** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

86. **[P2/Blocker] Event/default/kubepreflight-demo-control-plane.18c119c76c8eafa8** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

87. **[P2/Blocker] Event/demo/singleton-app-pdb.18c119cef691ea71** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

88. **[P2/Blocker] Event/demo/singleton-app-685c8555c8-8hvnd.18c119cf140ac89d** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

89. **[P2/Blocker] FlowSchema/system-leader-election** (API-001)

   ```
   Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

90. **[P2/Blocker] FlowSchema/system-node-high** (API-001)

   ```
   Migrate to flowcontrol.apiserver.k8s.io/v1 FlowSchema before upgrading past 1.26. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references flowcontrol.apiserver.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

91. **[P2/Blocker] Event/kube-system/coredns-57575c5f89-d2hrs.18c119cc66271e96** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

92. **[P2/Blocker] Event/demo/shared-app-pdb-a.18c119cf01801233** (API-001)

   ```
   Migrate to events.k8s.io/v1 Event before upgrading past 1.25. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. For Helm releases whose stored release manifest still references events.k8s.io/v1beta1, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.
   ```

93. **[P3/Blocker] Node/kubepreflight-demo-control-plane** (NODE-001)

   ```
   Replace this node (managed node group rolling update, Karpenter Drift, or manual AMI bump) to pick up a kubelet within the supported skew window before proceeding with the next control-plane minor version upgrade. Deferred control-plane upgrades compound this: each skipped bump narrows the legal skew window for the next one.
   ```

94. **[P4/Blocker] ValidatingWebhookConfiguration/demo-catchall-guard** (WH-001, WH-002)

   ```
   Step 1 — restore the webhook backend (read-only, safe to run any time):

   kubectl get svc 'dead-guard-svc' -n 'demo'
   kubectl get endpointslices -n 'demo' -l kubernetes.io/service-name='dead-guard-svc'
   kubectl get deploy,pods -n 'demo'

   Step 2 — only if you need immediate relief and cannot wait for the backend to recover:

   This TEMPORARILY REMOVES the webhook's protection. The "test" operation
   guards against the array index having shifted since this scan ran — the
   patch aborts instead of silently touching the wrong webhook block.

   kubectl patch validatingwebhookconfiguration 'demo-catchall-guard' --type='json' -p='[{"op":"test","path":"/webhooks/0/name","value":"guard.demo.kubepreflight.io"},{"op":"replace","path":"/webhooks/0/failurePolicy","value":"Ignore"}]'

   Revert failurePolicy to Fail immediately after the backend recovers.
   ```

   Also see `WH-001`: Inspect the webhook's current rules and selectors: ...

95. **[P4/Warning] ConfigMap/kube-system/coredns** (COREDNS-001)

   ```
   Add `ready` as a standalone directive inside the server block (typically alongside `health`). Back up the Corefile ConfigMap first, then apply the change directly with `kubectl apply` or via `aws eks update-addon --addon-name coredns --resolve-conflicts PRESERVE` if CoreDNS is managed as an EKS add-on.
   ```

96. **[P4/Warning] Deployment/local-path-storage/local-path-provisioner** (NODE-003)

   ```
   Replace deprecated node-role.kubernetes.io/master references with node-role.kubernetes.io/control-plane, or migrate to an explicit stable node label managed by the platform team. Validate that all target nodes already carry the replacement label before changing selectors or affinities — changing the selector first strands the workload with no schedulable nodes.
   ```

## Evidence Appendix

Every finding's resource identity and fingerprint — cross-reference by fingerprint for waivers/dedup.

| Priority | Rule ID | Severity | Confidence | Resource | Fingerprint |
|---|---|---|---|---|---|
| P2 | API-001 | Blocker | STATIC_CERTAIN | EndpointSlice/default/kubernetes | `643fd4125f1751b0abed7d7b33bb92b5bfd699ffd1af8dcdbcfa6907a75091f1` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | EndpointSlice/demo/dead-guard-svc-qrd2w | `b6bfe095082ce049f8be0752af56b0109993128efa4838de1f77d459dd85c855` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | EndpointSlice/kube-system/kube-dns-zj8ct | `b0772b602acc215206daa00d7a68f41c974e053dacde6d4f4c5f07b05ac110ab` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/default/kubepreflight-demo-control-plane.18c119c76c8eafa8 | `f9af9acb7b0cb1300f7270ea18c3aa8754fce8e0fef71781218a43b0d395e035` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/default/kubepreflight-demo-control-plane.18c119c770ee046d | `8fcb300fda531dbff1e3dada7aa4cf6a100e16f35e73903f322f95e1e9752e54` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/default/kubepreflight-demo-control-plane.18c119c772b49864 | `68c49aa960dc59eda480341ea6df299558448137656de72425f5f61ad77ed8c6` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/default/kubepreflight-demo-control-plane.18c119c772b4af6d | `1dab8d3399297e15b2a825cbece9bb3108604fbfd4bd39f492e5982ff4c82420` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/default/kubepreflight-demo-control-plane.18c119c772b4bc0e | `430a445ae37281829a09231e2b149a7efeb55dc34639f3ce34521002bacac769` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/default/kubepreflight-demo-control-plane.18c119ca5d558648 | `15690f1a28f8eaacb6c45b029c0e02edea655ed967b1134d0765fc34ec4a54d3` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/default/kubepreflight-demo-control-plane.18c119cab1f091a9 | `0fc96e3ead146e55b5634f08209e0734b935689428d086666b27cf0b69fee719` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/default/kubepreflight-demo-control-plane.18c119cc33fc383d | `4f213aec2fdfce0dc2e295b54001385b2ee6dd4ba0920b73daac000fcaf45d2b` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/demo/shared-app-5d96875494-9lh5m.18c119cf0229f07a | `9a03e504a4142287dcd0569fe9a279b982e495c77e19e4a99a6a477867766bd9` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/demo/shared-app-5d96875494-9lh5m.18c119cf022b20b3 | `f537eabad22ee6b1d38984a4926ffb6c19c4863dd5f408a548fa83c8b2c67b07` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/demo/shared-app-5d96875494-9lh5m.18c119cf1be3f882 | `e10220341f97cd80ebd86d9017b724203f03b1dc06cf6ac49d5fe879a6995410` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/demo/shared-app-5d96875494-9lh5m.18c119cf9043fdf1 | `d7a06315d46a2bbf1640c02bbbdf026993a59b938194e14abb7c9a81d628c19d` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/demo/shared-app-5d96875494-9lh5m.18c119cf91195da6 | `0e4f77c7d17d5c9532406d7f187dd2f53f34aa9512db5653d0b4fa3710e96c45` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/demo/shared-app-5d96875494-9lh5m.18c119cf9869caad | `de7013742280f2f8cbc163ec3a19566e7520faa13129c2196dbc600ae0cee43d` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/demo/shared-app-5d96875494-xnwsk.18c119cf01ce642e | `2ac126efaaf9664f785bfcfa907404c27780a6fa6f9801df1ab8f0c43d0c2daf` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/demo/shared-app-5d96875494-xnwsk.18c119cf02c32754 | `f54717dba0fc8a143edc07bc91b99a90bbc362d32f46c5ba54538a4307ee2fd6` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/demo/shared-app-5d96875494-xnwsk.18c119cf1b93d641 | `a0398f9628f57bf7993f041d1fcea7fba5fce2f1710596c7426073e94a3c51d0` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/demo/shared-app-5d96875494-xnwsk.18c119cf7e1cd35e | `52b950f9b3fbcfc0248731a3dc5dab57dea068f8e5e7c98afc13c7e7b9de8a09` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/demo/shared-app-5d96875494-xnwsk.18c119cf7ece15a3 | `1b2265374aa890c66679a294f41aee4cf3acfb45010e4a5c767187f8b03f0692` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/demo/shared-app-5d96875494-xnwsk.18c119cf86da0b40 | `847460524a5812c8cc72a8dfce8a704fb459d008e4551d8fdcd8579886cbb5bb` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/demo/shared-app-5d96875494.18c119cf0193feae | `1e902f13efa38264d32840f14683891733c6a8742decd895fa977a6f3ebb3c71` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/demo/shared-app-5d96875494.18c119cf01d77994 | `3bb81e1d7d89db7b5d4608277608800363093e5756dee31f5479ddbd76d3bc71` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/demo/shared-app-pdb-a.18c119cf01801233 | `aff74b1b553fa5d3510006ff90276e5ca651eeebc9d770726da86e5942353b1b` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/demo/shared-app.18c119cf01564e8c | `d7e105b75f14d1deca6819ab7be67748010bed04d04ad078a729ddb426b1e066` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/demo/singleton-app-685c8555c8-8hvnd.18c119cef6f9b5ad | `97f8f86dd75fb8d30eff1ce58650288821c8b26a1ddb19ffa810d7c84969c769` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/demo/singleton-app-685c8555c8-8hvnd.18c119cf140ac89d | `2e26231367079c14a15c63349bf0c9e8a853ce8abe8f8d81886076bd398d7056` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/demo/singleton-app-685c8555c8-8hvnd.18c119cf697d141c | `3daba7bac571f27241c8d5251a67baf322983b59b05da595b9e917e9a1ff1f0e` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/demo/singleton-app-685c8555c8-8hvnd.18c119cf6a4172c1 | `b60774dbd94a9116d696adf99c6183463f5fb1d580ff229432ea786b58335ab4` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/demo/singleton-app-685c8555c8-8hvnd.18c119cf7154a94b | `985dda9654b43f84f2c17dd86bb369f3e48848e3596f9ddbbc70c145172271e1` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/demo/singleton-app-685c8555c8.18c119cef6b308d4 | `108362dc62e19035387ff776770efa69de523375744ca4b1668053be17c30df2` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/demo/singleton-app-pdb.18c119cef691ea71 | `329a68e2075dbbed58e10e807835b2359594ec8fac8e30ff77152ba7142c4a9d` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/demo/singleton-app.18c119cef6415cc4 | `c2efc6dd1972625043eeb4710e6f592d908e1c1c5bd56682e9e04398ab77dc25` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/kube-system/coredns-57575c5f89-d2hrs.18c119ca944db7c1 | `9204ee1dbb2c17673d870e6529c29626f1ed2df62975beb55a51e9c7bcd3db59` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/kube-system/coredns-57575c5f89-d2hrs.18c119cc3bfbdf6f | `66c5e1cfa6428491c43880809250563b4b10f9013e2a03b431edd4aa0bc89f96` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/kube-system/coredns-57575c5f89-d2hrs.18c119cc66271e96 | `59c118f346bf1a05cf26fc1fe7711aaa6b860efe62e072a90f8244f0fa575ce6` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/kube-system/coredns-57575c5f89-d2hrs.18c119cc6698a27d | `56a2f84321a1d02a8ff778b073ed5d288ad0f90793a307e1d81219037c4c8367` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/kube-system/coredns-57575c5f89-d2hrs.18c119cc6d22c56a | `afa6e26f56f92b694f7affeb61514334f1a15db6a085b45c017bb4e0fd1955bb` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/kube-system/coredns-57575c5f89-d6dln.18c119ca938ddd34 | `87dabfdd0a6aabd933fbcfccf16492c9465e4c4bdbe9ecf91bdd9dfafafc1f49` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/kube-system/coredns-57575c5f89-d6dln.18c119cc3bf4a2a0 | `f95f934f627924a48d9f5a09ab36c39d83cefe2f878071699719c88446a6236b` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/kube-system/coredns-57575c5f89-d6dln.18c119cc5824cb45 | `91b82dc1554e6c544121c9387283323123a831fa99de574813f320898d9a7e2c` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/kube-system/coredns-57575c5f89-d6dln.18c119cc6369acf7 | `2830016939806483d2c89b857ede52dd006f95bc787878544aa824ef2e9b800c` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/kube-system/coredns-57575c5f89-d6dln.18c119cc686608b2 | `ae1a9e756ca63247087597a2062d8968fcf0bdec953072be88f681588124caee` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/kube-system/coredns-57575c5f89.18c119ca938b182c | `38228e8399d316160c7262733e22ae2704e0e15531d770a971ab5a983502028f` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/kube-system/coredns-57575c5f89.18c119ca93df13e6 | `c5f35fb5003ffdb0a858281b7e23964300a9208a87b8493736b8666f7fcb7f3b` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/kube-system/coredns.18c119ca60d30d59 | `c529f9127d3609bddf203bf3cae1cd09a81f81cd29056cb96afda586038fb8aa` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/kube-system/kindnet-cn6z9.18c119ca7ee82343 | `fc003019f7f81899489e41d9e93fc70ff3c6b5f71344dc57daa8e460ef72cbba` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/kube-system/kindnet-cn6z9.18c119caa9198485 | `5531f4a21bb35a394c3e08796955bd0fa41f29ad2900af4621e05afdc7b469f3` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/kube-system/kindnet-cn6z9.18c119cac5533665 | `0a3a542c52f5e6ef1f0ba5acbc298f0e02918a775da25defca6f516bf7fe18a6` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/kube-system/kindnet-cn6z9.18c119cafdcc084f | `69feb59558ff77e38ee0d270fca459f6a0a2c28522f5a72c660379c22ea073e6` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/kube-system/kindnet.18c119ca7ea12743 | `4bdfd143535a03b70ed447774b6a518f295cf653b64bdeb8273a0dbb493b5947` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/kube-system/kube-controller-manager.18c119c7584c6a46 | `81b20aa12aa64a2fb1b2add2496b561d495d31d113fc999832074fa24afab34f` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/kube-system/kube-proxy-bcnz7.18c119ca7ee61dca | `8a326fee4e7fb7a907659fec7d05a7bb85e4bfe49ae616bd9bcf33f40634a5cc` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/kube-system/kube-proxy-bcnz7.18c119ca98cf6471 | `307638812985dd3da401b21fe29c6ed793dd38ad74686fe331605ee73d787be6` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/kube-system/kube-proxy-bcnz7.18c119caa9374311 | `d8b97631de92385c4a50b747d1b49f20c3680fc4533c5761b249eb3cfc40d709` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/kube-system/kube-proxy-bcnz7.18c119caaee6ed7b | `189ddab2204ac68b71aba1dfb1eb4af5d41ac4a70df98208f0c66ac139104ad6` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/kube-system/kube-proxy.18c119ca7e978209 | `2e0d3558b0fdfd5dab12d9ee3aca38d70558c1e69da4cdc31dc239cf399351b4` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/kube-system/kube-scheduler.18c119c79d11cb6f | `6a378e245d3b7d9eed7c1772ab4ff6f729edb2999b86a543912695da0d2a833f` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/local-path-storage/local-path-provisioner-c49b7b56f-66nwj.18c119ca94ef7ad0 | `17837c8ce82670665c6f03fc10493e78efb37cc7c751eab1931c191187325cde` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/local-path-storage/local-path-provisioner-c49b7b56f-66nwj.18c119cc3bfc3d7e | `735d445975a1b60f7defc0126e4be8edfb4fdefd3f8a856a0651dc4a79f3069b` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/local-path-storage/local-path-provisioner-c49b7b56f-66nwj.18c119cc58501ecd | `424664202252d4e7bac6f1a1808efa257200bea029656d055acba88235596615` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/local-path-storage/local-path-provisioner-c49b7b56f-66nwj.18c119cc67b63df1 | `52f18b1b82615944b4d007d71fa583a6f178321828705a03a3a64bfc15b95aca` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/local-path-storage/local-path-provisioner-c49b7b56f-66nwj.18c119cc6d5f0d3c | `0cd49bf7c7ba6051a551dd014122f57a23017e22fdb28ee85f0d0a238d5213d1` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/local-path-storage/local-path-provisioner-c49b7b56f.18c119ca93a732bd | `0f7c8ebcb8f8d6492c1065cd0e040b73493102e4e315df340bc6f00b2f4021e0` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | Event/local-path-storage/local-path-provisioner.18c119ca60d2bb69 | `7b8bb0251427d0a0d46da77a67fe92b3aed0a43a37babc578575dec6e90e6bb0` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | FlowSchema/catch-all | `78f23eb2d83cde3ff603baa09ea083337b2d32432eb17898443a0d99277e0a82` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | FlowSchema/endpoint-controller | `6479a5d5e9e68faf4d4d8e0531b90ded996538a4440e537e09b72c13e8a6803d` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | FlowSchema/exempt | `86ef3a38f523ec3bdfa47eebc31186692ebd83ce4a73eda087af383e2438b42a` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | FlowSchema/global-default | `c061bcf147137479355a08616e055505141182fac40c0c1a19b1432ce5055a66` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | FlowSchema/kube-controller-manager | `dd27bd14aab9c7a18049b5aab3a569014639aac9697264dddb5248b87c5ffd74` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | FlowSchema/kube-scheduler | `998e79687e9a1b03acfa07db8466063a02505dbcf72a60174782ddd08dfbfd7a` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | FlowSchema/kube-system-service-accounts | `dc79de296042918db4b0ebf981736b36ec5b1eea50f2af1b8a38f5048a2258a6` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | FlowSchema/probes | `9caddeb1440296a1dcc1aebe8e50c1a5d2c745cb4ed9b364aad8a325d3c5a107` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | FlowSchema/service-accounts | `d4b4f01299c45693f2e6b4ae85b244cf1f79c314f6a43ae697cc85b2672a3436` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | FlowSchema/system-leader-election | `8c164dbcd1c98949b5653b3658b38e0638072a09cdb612b68b37bbfbb1d6ecb0` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | FlowSchema/system-node-high | `dc7aede40ea39bbe4cfa472a65a4af9ac930d99ca8e5dd63c74284b2e5e5e4f6` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | FlowSchema/system-nodes | `4b7ccae5314c730de3929cce1b91c74c8b2dc8042eb293409a9a933aa5a775c2` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | FlowSchema/workload-leader-election | `00216829b4a86cf41a9f27a3ec8f4243fd18f7a3147e280629a74eebcbf830e0` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | PodDisruptionBudget/demo/shared-app-pdb-a | `36cb521e132bc4faeb8ded5709d52993da4ed27eac7574604f01bad5982fc04b` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | PodDisruptionBudget/demo/shared-app-pdb-b | `06f186afc01178e8032232dfa3864060bc8525fcadfb2105089706d3d3bef74c` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | PodDisruptionBudget/demo/singleton-app-pdb | `cb804097cb7f43763a1b413ae9502ab9d9de518bac7ecca9346979f3e9d1baab` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | PodSecurityPolicy/demo-restricted | `10646905cb023d48e9918c262298e2c489572d7176d7b3eb2bbcb9ce366ebcbf` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | PriorityLevelConfiguration/catch-all | `4e90d8dd58979547bd8a04325706539963ad3f31ea15224bc713b1ea292e0053` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | PriorityLevelConfiguration/exempt | `6cb7bc15020705fba185bb031fed5f32e7e53fde72e721d3371fb8959e7665b1` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | PriorityLevelConfiguration/global-default | `dae73ff30320491abefd07d6b2eb5bde04edb1d2e578cff6e6d3f2a37fdd7c4d` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | PriorityLevelConfiguration/leader-election | `d48c3b78246556cae2920ad6f86cfd08fdc684817b96f695c1983316a53df608` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | PriorityLevelConfiguration/node-high | `513d2906589241f21aa36753dee3cc55f53e05e153714604b0b4af5432c5c866` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | PriorityLevelConfiguration/system | `8c9dc9f16d05ac40d9387b718382bab7ba428f8744ed12c7eedf093ae3929d31` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | PriorityLevelConfiguration/workload-high | `db706f56a9e2b15e4886d3f33783a2b31790661857d03f6e8670eb070b383844` |
| P2 | API-001 | Blocker | STATIC_CERTAIN | PriorityLevelConfiguration/workload-low | `20be091261761ef98c7d8c3715e2b2c217cc6f2f8da7d6935e1be6f38671e441` |
| P2 | NODE-003 | Blocker | STATIC_CERTAIN | Deployment/kube-system/coredns | `6a7a8014f0ad5c824bd8672b662a4189c03bcf048a95f268f24309d1c8a61147` |
| P3 | NODE-001 | Blocker | STATIC_CERTAIN | Node/kubepreflight-demo-control-plane | `7aa0e34b95afe9aaa612f109f25a4503ba7994490892f7216e177a5452036412` |
| P3 | PDB-001 | Blocker | OBSERVED | PodDisruptionBudget/demo/shared-app-pdb-b | `33f264ee826fd87a2a0ab55c30a657930705297a79be08315a0667a4b15209ff` |
| P3 | PDB-001 | Blocker | OBSERVED | PodDisruptionBudget/demo/singleton-app-pdb | `d20021d57af2f301b8befe8102b85385c1edf52a4106dfeec9bc4c5402b5620f` |
| P3 | PDB-002 | Blocker | OBSERVED | PodDisruptionBudget/demo/shared-app-pdb-a,shared-app-pdb-b | `aefe0d92b0b59172167dab47f123d29e7a83fa89065f964821fee77f4d079a6e` |
| P4 | WH-002 | Blocker | OBSERVED | ValidatingWebhookConfiguration/demo-catchall-guard | `d9168e90cde35088961fc18afe2951825607e6651f7b5e34896cdaab5144df1e` |
| P4 | COREDNS-001 | Warning | STATIC_CERTAIN | ConfigMap/kube-system/coredns | `a420e3934b41962784d0bf10fde52b351c1aeb655660df0cef7b669223fb9ba7` |
| P4 | NODE-003 | Warning | STATIC_CERTAIN | Deployment/local-path-storage/local-path-provisioner | `a77880b57e390696b5490912886b2a554a6fc65927591e62ce9e962a15b7946e` |
| P4 | WH-001 | Warning | STATIC_CERTAIN | ValidatingWebhookConfiguration/demo-catchall-guard | `b0d3e70e7b6d1a7ef56651ab578150fdc8935dbf332e1bcbc3dda69addc02721` |

