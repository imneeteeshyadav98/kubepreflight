# KubePreflight Rollback Readiness

| | |
|---|---|
| **Schema** | `kubepreflight.io/rollback-assessment/v1alpha1` |
| **Mode** | `post_upgrade_readiness` |
| **Cluster** | kubepreflight-case-study |
| **Region** | us-east-1 |
| **Current version** | 1.32 |
| **Rollback target** | 1.31 |
| **Eligibility** | `eligible` |
| **Readiness** | `blocked` |
| **Recommendation** | `do_not_proceed` |
| **Confidence** | `high` |
| **Evidence complete** | No |
| **Rollback window** | at least 167h 48m remaining |

## Reason Codes

- `END_OF_EXTENDED_SUPPORT_AUTO_UPGRADE_UNVERIFIED`
- `EKS_FEATURE_COMPATIBILITY_UNVERIFIED`
- `MANAGED_ADDON_COMPATIBILITY_UNKNOWN`
- `SELF_MANAGED_ADDON_COMPATIBILITY_UNKNOWN`
- `UNHEALTHY_WORKLOADS_PRESENT`
- `PDB_DISRUPTION_CONSTRAINTS`
- `NEW_VERSION_API_ADOPTION_RISK`
- `CRD_WEBHOOK_CONTROLLER_RISK`

## Checks

| Check | Status | Reason codes | Evidence |
|---|---|---|---|
| EKS cluster status is ACTIVE | `pass` | none | status: ACTIVE |
| Rollback target EKS version is supported | `pass` | none | target version: 1.31<br>target versionStatus: EXTENDED_SUPPORT |
| Cluster upgrade policy allows extended-support rollback target | `pass` | none | upgrade policy supportType: EXTENDED<br>target versionStatus: EXTENDED_SUPPORT |
| Previous version is exactly N-1 | `pass` | none | current version: 1.32<br>rollback target version: 1.31 |
| EKS rollback window is active | `pass` | none | upgrade update id: 9b2fe685-ac09-3dcc-96b6-66b3d7921784<br>upgrade update createdAt: 2026-07-16T17:04:01Z<br>window calculation: conservative<br>timestamp source: eks_update_created_at |
| End-of-extended-support auto-upgrade origin is not yet verified | `unknown` | END_OF_EXTENDED_SUPPORT_AUTO_UPGRADE_UNVERIFIED | none |
| Backward-incompatible EKS feature compatibility is not yet verified | `unknown` | EKS_FEATURE_COMPATIBILITY_UNVERIFIED | none |
| Managed node groups are compatible with rollback target | `pass` | none | nodegroup ng-small version: 1.31 status: ACTIVE |
| Self-managed and hybrid node evidence is available | `pass` | none | kubernetes coverage: complete |
| Fargate rollback implications are identified | `pass` | none | No Fargate-specific findings present in current evidence |
| EKS managed add-ons are compatible with rollback target | `warning` | MANAGED_ADDON_COMPATIBILITY_UNKNOWN | addon coredns version: v1.11.4-eksbuild.39 compatible: true verificationUnavailable: false<br>addon kube-proxy version: v1.31.14-eksbuild.18 compatible: true verificationUnavailable: false<br>addon vpc-cni version: v1.21.2-eksbuild.2 compatible: true verificationUnavailable: false<br>ADDON-002 Warning: EKS add-on "coredns" version v1.11.4-eksbuild.39 has no compatibility catalog entry for target Kubernetes 1.32 — confirm compatibility before starting the upgrade<br>ADDON-002 Warning: EKS add-on "kube-proxy" version v1.31.14-eksbuild.18 has no compatibility catalog entry for target Kubernetes 1.32 — confirm compatibility before starting the upgrade<br>ADDON-002 Warning: EKS add-on "vpc-cni" version v1.21.2-eksbuild.2 has no compatibility catalog entry for target Kubernetes 1.32 — confirm compatibility before starting the upgrade |
| Self-managed add-on rollback compatibility is verified | `warning` | SELF_MANAGED_ADDON_COMPATIBILITY_UNKNOWN | ADDON-002 Warning: EKS add-on "coredns" version v1.11.4-eksbuild.39 has no compatibility catalog entry for target Kubernetes 1.32 — confirm compatibility before starting the upgrade<br>ADDON-002 Warning: EKS add-on "kube-proxy" version v1.31.14-eksbuild.18 has no compatibility catalog entry for target Kubernetes 1.32 — confirm compatibility before starting the upgrade<br>ADDON-002 Warning: EKS add-on "vpc-cni" version v1.21.2-eksbuild.2 has no compatibility catalog entry for target Kubernetes 1.32 — confirm compatibility before starting the upgrade |
| Workloads are healthy before rollback | `warning` | UNHEALTHY_WORKLOADS_PRESENT | WORKLOAD-001 Warning: Workload has unhealthy pods before upgrade: 1 pod in ImagePullBackOff. This workload was unhealthy before the upgrade, which can make post-upgrade validation ambiguous. |
| PDB and drain constraints do not block rollback preparation | `warning` | PDB_DISRUPTION_CONSTRAINTS | DRAIN-001 Warning: Deployment preflight-case-study/already-broken-app runs a single replica (desired: 1, ready: 0, available: 0) — when its pod is evicted (node drain, node upgrade, or any voluntary disruption), this workload has zero available replicas until a replacement schedules and becomes Ready elsewhere; no PodDisruptionBudget protects this workload<br>DRAIN-003 Warning: Deployment kube-system/coredns has a nodeSelector/required nodeAffinity satisfied by only 1 node(s) in this cluster today (ip-10-0-1-100.ec2.internal) — if that node is drained, no other currently-known node can host a replacement pod |
| API, CRD, and webhook state is compatible with rollback target | `fail` | NEW_VERSION_API_ADOPTION_RISK, CRD_WEBHOOK_CONTROLLER_RISK | API-001 Blocker: PodDisruptionBudget "default/old-pdb-api" (apiVersion policy/v1beta1) in old-api.yaml uses an API version removed in Kubernetes 1.25 — this manifest will fail to apply once the cluster reaches target 1.32<br>WH-005 Warning: ValidatingWebhookConfiguration "vpc-resource-validating-webhook": webhook "vnode.vpc.k8s.aws" (index 1 in .webhooks) matches nodes — a fail-closed webhook here can block node status updates, namespace lifecycle, or PersistentVolume operations that upgrade/maintenance workflows depend on |
| Operational evidence coverage is complete | `pass` | none | kubernetes coverage: complete<br>aws coverage: complete<br>manifest coverage: complete |

