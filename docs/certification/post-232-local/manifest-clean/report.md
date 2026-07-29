# KubePreflight Scan Report

| | |
|---|---|
| **Cluster** | - |
| **Target version** | 1.34 |
| **Provider** | cluster-only |
| **Upgrade context** | unspecified |
| **Scanned at** | 2026-07-29 07:32:13 UTC |
| **Result** | **CLEAN** |
| **Summary** | 0 blocker(s), 0 warning(s), 0 operator decision(s), 0 info(s) |

> **Manifest-only result:** Manifest API checks clean. Cluster, AWS, scheduling, disruption, add-on, node, CRD, and webhook checks were not evaluated in manifest-only mode.

## Upgrade Readiness

| | |
|---|---|
| **Verdict** | CLEAN |
| **Readiness score** | 100/100 |
| **Upgrade continue** | Yes |

| Category | Status | Blockers | Warnings | Rule IDs |
|---|---|---|---|---|
| API Compatibility | Passed | 0 | 0 |  |
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
| **Status** | Passed |
| **Upgrade continue** | Yes |
| **Score impact** | 0 |
| **Removed API objects** | 0 across 0 API families |
| **Deprecated API objects** | 0 across 0 API families |
| **Critical impact** | No |

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
| `API-001` | Applicable | Evaluated | No issue detected |  |
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
