# KubePreflight Rollback Readiness

| | |
|---|---|
| **Schema** | `kubepreflight.io/rollback-assessment/v1alpha1` |
| **Mode** | `post_upgrade_readiness` |
| **Cluster** | kp-cert-0729-a0fb234a |
| **Region** | us-east-1 |
| **Current version** | 1.34 |
| **Rollback target** | 1.33 |
| **Eligibility** | `unavailable` |
| **Readiness** | `insufficient_evidence` |
| **Recommendation** | `do_not_proceed` |
| **Confidence** | `high` |
| **Evidence complete** | No |
| **Rollback window** | unknown |

## Reason Codes

- `EKS_UPGRADE_HISTORY_UNAVAILABLE`
- `END_OF_EXTENDED_SUPPORT_AUTO_UPGRADE_UNVERIFIED`
- `EKS_FEATURE_COMPATIBILITY_UNVERIFIED`
- `MANAGED_NODEGROUP_ROLLBACK_REQUIRED`
- `PDB_DISRUPTION_CONSTRAINTS`
- `ROLLBACK_EVIDENCE_TARGET_MISMATCH`

## Checks

| Check | Status | Reason codes | Evidence |
|---|---|---|---|
| EKS cluster status is ACTIVE | `pass` | none | status: ACTIVE |
| Rollback target EKS version is supported | `pass` | none | target version: 1.33<br>target versionStatus: EXTENDED_SUPPORT |
| Cluster upgrade policy allows extended-support rollback target | `pass` | none | upgrade policy supportType: EXTENDED<br>target versionStatus: EXTENDED_SUPPORT |
| Previous version is exactly N-1 | `pass` | none | current version: 1.34<br>rollback target version: 1.33 |
| EKS rollback window is active | `unknown` | EKS_UPGRADE_HISTORY_UNAVAILABLE | none |
| End-of-extended-support auto-upgrade origin is not yet verified | `unknown` | END_OF_EXTENDED_SUPPORT_AUTO_UPGRADE_UNVERIFIED | none |
| Backward-incompatible EKS feature compatibility is not yet verified | `unknown` | EKS_FEATURE_COMPATIBILITY_UNVERIFIED | none |
| Managed node groups are compatible with rollback target | `warning` | MANAGED_NODEGROUP_ROLLBACK_REQUIRED | nodegroup cert-ng2 version: 1.34 status: ACTIVE |
| Self-managed and hybrid node evidence is available | `pass` | none | kubernetes coverage: complete |
| Fargate rollback implications are identified | `pass` | none | No Fargate-specific findings present in current evidence |
| EKS managed add-ons are compatible with rollback target | `pass` | none | addon coredns version: v1.12.4-eksbuild.18 compatible: true verificationUnavailable: false<br>addon kube-proxy version: v1.34.6-eksbuild.17 compatible: true verificationUnavailable: false<br>addon vpc-cni version: v1.22.3-eksbuild.1 compatible: true verificationUnavailable: false |
| Self-managed add-on rollback compatibility is verified | `pass` | none | No self-managed add-on compatibility warnings present |
| Workloads are healthy before rollback | `pass` | none | No unhealthy workload findings present |
| PDB and drain constraints do not block rollback preparation | `warning` | PDB_DISRUPTION_CONSTRAINTS | DRAIN-003 Warning: Deployment kube-system/coredns has a nodeSelector/required nodeAffinity satisfied by only 1 node(s) in this cluster today ([redacted-node-hostname]) — if that node is drained, no other currently-known node can host a replacement pod |
| API, CRD, and webhook state is compatible with rollback target | `unknown` | ROLLBACK_EVIDENCE_TARGET_MISMATCH | CRD/webhook compatibility evidence target mismatch: findings target=1.34 rollback target=1.33 |
| Operational evidence coverage is complete | `pass` | none | kubernetes coverage: complete<br>aws coverage: complete<br>manifest coverage: skipped |
