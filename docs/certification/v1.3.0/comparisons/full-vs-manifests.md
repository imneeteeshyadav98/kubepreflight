# Upgrade Readiness Comparison

- Schema version: `kubepreflight.io/scan-comparison/v1`

| | |
|---|---|
| **Verdict** | PASSED_WITH_WARNINGS → BLOCKED |
| **Upgrade context** | unspecified → unspecified |
| **Readiness score** | 77 → 85 (+8) |
| **New** | 1 (1 blocker(s)) |
| **Resolved** | 0 (0 blocker(s)) |
| **Not re-evaluated** | 8 |
| **Changed** | 0 |
| **Unchanged** | 0 |

## New findings (1)

| Priority | Severity | Rule | Resource | Message |
|---|---|---|---|---|
| P2 | Blocker | `API-001` | default/old-pdb-api | PodDisruptionBudget "default/old-pdb-api" (apiVersion policy/v1beta1) in old-api.yaml uses an API version removed in Kubernetes 1.25 — this manifest will fail to apply once the cluster reaches target 1.36 |

## Changed findings (0)

None.

## Resolved findings (0)

None.

## Not re-evaluated findings (8)

The finding was present in the baseline, but its rule was not successfully evaluated in the current report, so resolution cannot be confirmed.

| Priority | Severity | Rule | Resource | Message |
|---|---|---|---|---|
| P3 | Warning | `ADDON-002` | coredns | EKS add-on "coredns" version v1.14.2-eksbuild.4 has no compatibility catalog entry for target Kubernetes 1.36 — confirm compatibility before starting the upgrade |
| P3 | Warning | `ADDON-002` | kube-proxy | EKS add-on "kube-proxy" version v1.36.0-eksbuild.7 has no compatibility catalog entry for target Kubernetes 1.36 — confirm compatibility before starting the upgrade |
| P3 | Warning | `ADDON-002` | vpc-cni | EKS add-on "vpc-cni" version v1.21.2-eksbuild.2 has no compatibility catalog entry for target Kubernetes 1.36 — confirm compatibility before starting the upgrade |
| P3 | Warning | `DRAIN-003` | kube-system/coredns | Deployment kube-system/coredns has a nodeSelector/required nodeAffinity satisfied by only 1 node(s) in this cluster today (<NODE_PRIVATE_HOSTNAME>) — if that node is drained, no other currently-known node can host a replacement pod |
| P3 | Warning | `DRAIN-003` | kube-system/metrics-server | Deployment kube-system/metrics-server has a nodeSelector/required nodeAffinity satisfied by only 1 node(s) in this cluster today (<NODE_PRIVATE_HOSTNAME>) — if that node is drained, no other currently-known node can host a replacement pod |
| P3 | Warning | `EKS-NG-002` | certification-ng | Managed node group "certification-ng" desired size equals or is below minimum size. Rolling update may have limited disruption headroom. |
| P4 | Warning | `WH-005` | vpc-resource-validating-webhook | ValidatingWebhookConfiguration "vpc-resource-validating-webhook": webhook "vnode.vpc.k8s.aws" (index 1 in .webhooks) matches nodes — this webhook covers node status updates, namespace lifecycle, or PersistentVolume operations but is fail-open (failurePolicy: Ignore), so an unavailable backend won't block those operations; confirm fail-open was an intentional choice for a webhook covering resources this sensitive |
| P4 | Info | `EKS-NG-003` | certification-ng | Managed node group "certification-ng" uses a launch template/custom AMI. Validate AMI, bootstrap, kubelet, and launch template upgrade path manually. |

## Unchanged findings (0)

None.
