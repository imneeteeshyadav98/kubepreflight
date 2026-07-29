# KubePreflight Scan Report

| | |
|---|---|
| **Cluster** | - |
| **Target version** | 1.25 |
| **Provider** | cluster-only |
| **Upgrade context** | unspecified |
| **Scanned at** | 2026-07-29 07:32:13 UTC |
| **Result** | **BLOCKED** |
| **Summary** | 1 blocker(s), 0 warning(s), 0 operator decision(s), 0 info(s) |

## Upgrade Readiness

| | |
|---|---|
| **Verdict** | BLOCKED |
| **Readiness score** | 85/100 |
| **Upgrade continue** | No |

| Category | Status | Blockers | Warnings | Rule IDs |
|---|---|---|---|---|
| API Compatibility | Failed | 1 | 0 | API-001 |
| Extension APIs | Passed | 0 | 0 |  |
| Admission Webhooks | Passed | 0 | 0 |  |
| Disruption Safety | Passed | 0 | 0 |  |
| Drain Readiness | Passed | 0 | 0 |  |
| Node Readiness | Passed | 0 | 0 |  |
| Add-ons | Passed | 0 | 0 |  |
| CoreDNS | Passed | 0 | 0 |  |
| Workload Health | Passed | 0 | 0 |  |
| EKS Upgrade Insights | Passed | 0 | 0 |  |

## API Compatibility

| | |
|---|---|
| **Status** | Failed |
| **Upgrade continue** | No |
| **Score impact** | -40 |
| **Removed API objects** | 1 across 1 API family |
| **Deprecated API objects** | 0 across 0 API families |
| **Critical impact** | Yes |

### Removed API families

| API version | Kind | Objects |
|---|---|---|
| policy/v1beta1 | PodSecurityPolicy | 1 |

## Evaluation Coverage

| | |
|---|---|
| **Evaluation coverage** | Complete |
| **Total rules** | 31 |
| **Evaluated** | 2 |
| **Not evaluated** | 0 |
| **Insufficient evidence** | 0 |
| **Failed** | 0 |
| **Not applicable** | 29 |
| **Source** | Native |

| Rule ID | Applicability | Execution state | Outcome | Reason |
|---|---|---|---|---|
| `ADDON-001` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `ADDON-002` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `API-001` | Applicable | Evaluated | 1 finding(s) reported |  |
| `API-002` | Applicable | Evaluated | No issue detected |  |
| `APISERVICE-001` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `COREDNS-001` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `CRD-001` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `CRD-002` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `DRAIN-001` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `DRAIN-002` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `DRAIN-003` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `DRAIN-004` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `DRAIN-005` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `EKS-INSIGHT-001` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `EKS-INSIGHT-002` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `EKS-INSIGHT-003` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `EKS-NG-001` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `EKS-NG-002` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `EKS-NG-003` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `EKS-NG-004` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `NET-002` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `NODE-001` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `NODE-002` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `NODE-003` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `PDB-001` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `PDB-002` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `WH-001` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `WH-002` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `WH-004` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `WH-005` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |
| `WORKLOAD-001` | Not applicable | Not evaluated | Not applicable | not registered for this scan mode |

## Blockers (1)

### `P2` `API-001` PodSecurityPolicy "manifest-restricted" (apiVersion policy/v1beta1) in psp.yaml uses an API version removed in Kubernetes 1.25 — this manifest will fail to apply once the cluster reaches target 1.25

Confidence: `STATIC_CERTAIN` · Upgrade gate: `block` · Impact scope: `-` · Can upgrade continue: No

> **Why this matters (P2):** Resource or behavior may fail after the target Kubernetes upgrade.

**Evidence:**

- apiVersion: policy/v1beta1
- removed in: Kubernetes 1.25
- target version: 1.25
- source: psp.yaml
- catalog source: Kubernetes Deprecated API Migration Guide
- catalog reference: https://kubernetes.io/docs/reference/using-api/deprecation-guide/#podsecuritypolicy-v125

**Remediation:**

```
Migrate to Pod Security Admission or a policy engine (Kyverno/Gatekeeper) before this manifest is ever applied to a cluster at or past 1.25. Update and validate the source manifest against the replacement schema. For Helm charts, update the template itself — bumping the chart version alone doesn't help if the template source still emits the old apiVersion.
```

## Next Actions (1)

1. **[P2/Blocker] PodSecurityPolicy/manifest-restricted** (API-001)

   ```
   Migrate to Pod Security Admission or a policy engine (Kyverno/Gatekeeper) before this manifest is ever applied to a cluster at or past 1.25. Update and validate the source manifest against the replacement schema. For Helm charts, update the template itself — bumping the chart version alone doesn't help if the template source still emits the old apiVersion.
   ```

## Evidence Appendix

Every finding's resource identity and fingerprint — cross-reference by fingerprint for waivers/dedup.

| Priority | Rule ID | Severity | Confidence | Resource | Fingerprint |
|---|---|---|---|---|---|
| P2 | API-001 | Blocker | STATIC_CERTAIN | PodSecurityPolicy/manifest-restricted | `ad94d3c6d1a2d2c254e6a7004caceb788b339aa040258839b394d0e2450e9cda` |
