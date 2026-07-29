# KubePreflight Rollback Readiness

| | |
|---|---|
| **Schema** | `kubepreflight.io/rollback-assessment/v1alpha1` |
| **Mode** | `post_upgrade_readiness` |
| **Cluster** | dummy |
| **Region** | us-east-1 |
| **Current version** | - |
| **Rollback target** | - |
| **Eligibility** | `unknown` |
| **Readiness** | `insufficient_evidence` |
| **Recommendation** | `operator_decision_required` |
| **Confidence** | `low` |
| **Evidence complete** | No |
| **Rollback window** | unknown |

## Reason Codes

- `EKS_UPGRADE_HISTORY_UNAVAILABLE`
- `EKS_INSIGHTS_UNAVAILABLE`
- `ROLLBACK_EVIDENCE_CLUSTER_MISMATCH`
- `ROLLBACK_EVIDENCE_STALE`

## Checks

| Check | Status | Reason codes | Evidence |
|---|---|---|---|
| EKS rollback readiness insights are available | `unknown` | EKS_INSIGHTS_UNAVAILABLE | none |
| Managed node groups are compatible with rollback target | `unknown` | ROLLBACK_EVIDENCE_CLUSTER_MISMATCH, ROLLBACK_EVIDENCE_STALE | cluster identity mismatch: findings cluster=kubepreflight-v130-cert region=ap-south-1 vs assessed cluster=dummy region=us-east-1<br>findings evidence stale: scanned at 2026-07-27T09:50:07Z, evaluated at 2026-07-29T07:33:10Z, age 45h43m3s exceeds the 24h0m0s maximum |
| Self-managed and hybrid node evidence is available | `pass` | none | kubernetes coverage: complete |
| Fargate rollback implications are identified | `pass` | none | No Fargate-specific findings present in current evidence |
| EKS managed add-ons are compatible with rollback target | `unknown` | ROLLBACK_EVIDENCE_CLUSTER_MISMATCH, ROLLBACK_EVIDENCE_STALE | cluster identity mismatch: findings cluster=kubepreflight-v130-cert region=ap-south-1 vs assessed cluster=dummy region=us-east-1<br>findings evidence stale: scanned at 2026-07-27T09:50:07Z, evaluated at 2026-07-29T07:33:10Z, age 45h43m3s exceeds the 24h0m0s maximum |
| Self-managed add-on rollback compatibility is verified | `unknown` | ROLLBACK_EVIDENCE_CLUSTER_MISMATCH, ROLLBACK_EVIDENCE_STALE | cluster identity mismatch: findings cluster=kubepreflight-v130-cert region=ap-south-1 vs assessed cluster=dummy region=us-east-1<br>findings evidence stale: scanned at 2026-07-27T09:50:07Z, evaluated at 2026-07-29T07:33:10Z, age 45h43m3s exceeds the 24h0m0s maximum |
| Workloads are healthy before rollback | `unknown` | ROLLBACK_EVIDENCE_CLUSTER_MISMATCH, ROLLBACK_EVIDENCE_STALE | cluster identity mismatch: findings cluster=kubepreflight-v130-cert region=ap-south-1 vs assessed cluster=dummy region=us-east-1<br>findings evidence stale: scanned at 2026-07-27T09:50:07Z, evaluated at 2026-07-29T07:33:10Z, age 45h43m3s exceeds the 24h0m0s maximum |
| PDB and drain constraints do not block rollback preparation | `unknown` | ROLLBACK_EVIDENCE_CLUSTER_MISMATCH, ROLLBACK_EVIDENCE_STALE | cluster identity mismatch: findings cluster=kubepreflight-v130-cert region=ap-south-1 vs assessed cluster=dummy region=us-east-1<br>findings evidence stale: scanned at 2026-07-27T09:50:07Z, evaluated at 2026-07-29T07:33:10Z, age 45h43m3s exceeds the 24h0m0s maximum |
| API, CRD, and webhook state is compatible with rollback target | `unknown` | ROLLBACK_EVIDENCE_CLUSTER_MISMATCH, ROLLBACK_EVIDENCE_STALE | cluster identity mismatch: findings cluster=kubepreflight-v130-cert region=ap-south-1 vs assessed cluster=dummy region=us-east-1<br>findings evidence stale: scanned at 2026-07-27T09:50:07Z, evaluated at 2026-07-29T07:33:10Z, age 45h43m3s exceeds the 24h0m0s maximum |
| Operational evidence coverage is complete | `pass` | none | kubernetes coverage: complete<br>aws coverage: complete<br>manifest coverage: skipped |
