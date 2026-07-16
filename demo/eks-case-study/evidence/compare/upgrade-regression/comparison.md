# Upgrade Readiness Comparison

- Schema version: `kubepreflight.io/scan-comparison/v1`

| | |
|---|---|
| **Verdict** | BLOCKED |
| **Readiness score** | 57 → 57 (0) |
| **New** | 0 (0 blocker(s)) |
| **Resolved** | 26 (0 blocker(s)) |
| **Changed** | 0 |
| **Unchanged** | 12 |

## New findings (0)

None.

## Changed findings (0)

None.

## Resolved findings (26)

| Priority | Severity | Rule | Resource | Message |
|---|---|---|---|---|
| P2 | Info | `API-001` | catch-all | FlowSchema "catch-all" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |
| P2 | Info | `API-001` | catch-all | PriorityLevelConfiguration "catch-all" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |
| P2 | Info | `API-001` | eks-exempt | FlowSchema "eks-exempt" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |
| P2 | Info | `API-001` | eks-leader-election | FlowSchema "eks-leader-election" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |
| P2 | Info | `API-001` | eks-monitoring | PriorityLevelConfiguration "eks-monitoring" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |
| P2 | Info | `API-001` | eks-monitoring | FlowSchema "eks-monitoring" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |
| P2 | Info | `API-001` | eks-workload-high | FlowSchema "eks-workload-high" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |
| P2 | Info | `API-001` | endpoint-controller | FlowSchema "endpoint-controller" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |
| P2 | Info | `API-001` | exempt | FlowSchema "exempt" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |
| P2 | Info | `API-001` | exempt | PriorityLevelConfiguration "exempt" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |
| P2 | Info | `API-001` | global-default | PriorityLevelConfiguration "global-default" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |
| P2 | Info | `API-001` | global-default | FlowSchema "global-default" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |
| P2 | Info | `API-001` | kube-controller-manager | FlowSchema "kube-controller-manager" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |
| P2 | Info | `API-001` | kube-scheduler | FlowSchema "kube-scheduler" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |
| P2 | Info | `API-001` | kube-system-service-accounts | FlowSchema "kube-system-service-accounts" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |
| P2 | Info | `API-001` | leader-election | PriorityLevelConfiguration "leader-election" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |
| P2 | Info | `API-001` | node-high | PriorityLevelConfiguration "node-high" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |
| P2 | Info | `API-001` | probes | FlowSchema "probes" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |
| P2 | Info | `API-001` | service-accounts | FlowSchema "service-accounts" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |
| P2 | Info | `API-001` | system | PriorityLevelConfiguration "system" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |
| P2 | Info | `API-001` | system-leader-election | FlowSchema "system-leader-election" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |
| P2 | Info | `API-001` | system-node-high | FlowSchema "system-node-high" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |
| P2 | Info | `API-001` | system-nodes | FlowSchema "system-nodes" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |
| P2 | Info | `API-001` | workload-high | PriorityLevelConfiguration "workload-high" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |
| P2 | Info | `API-001` | workload-leader-election | FlowSchema "workload-leader-election" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |
| P2 | Info | `API-001` | workload-low | PriorityLevelConfiguration "workload-low" (apiVersion flowcontrol.apiserver.k8s.io/v1beta3) is an apiserver/platform-managed default that still exists at a version removed in Kubernetes 1.32 — usually no direct user action, since kube-apiserver or the cloud provider's control plane recreates its own flowcontrol defaults at the version it currently serves |

## Unchanged findings (12)

| Priority | Severity | Rule | Resource | Message |
|---|---|---|---|---|
| P2 | Blocker | `API-001` | default/old-pdb-api | PodDisruptionBudget "default/old-pdb-api" (apiVersion policy/v1beta1) in old-api.yaml uses an API version removed in Kubernetes 1.25 — this manifest will fail to apply once the cluster reaches target 1.32 |
| P3 | Warning | `ADDON-002` | coredns | EKS add-on "coredns" version v1.11.4-eksbuild.39 has no compatibility catalog entry for target Kubernetes 1.32 — confirm compatibility before starting the upgrade |
| P3 | Warning | `ADDON-002` | kube-proxy | EKS add-on "kube-proxy" version v1.31.14-eksbuild.18 has no compatibility catalog entry for target Kubernetes 1.32 — confirm compatibility before starting the upgrade |
| P3 | Warning | `ADDON-002` | vpc-cni | EKS add-on "vpc-cni" version v1.21.2-eksbuild.2 has no compatibility catalog entry for target Kubernetes 1.32 — confirm compatibility before starting the upgrade |
| P3 | Warning | `DRAIN-001` | preflight-case-study/already-broken-app | Deployment preflight-case-study/already-broken-app runs a single replica (desired: 1, ready: 0, available: 0) — when its pod is evicted (node drain, node upgrade, or any voluntary disruption), this workload has zero available replicas until a replacement schedules and becomes Ready elsewhere; no PodDisruptionBudget protects this workload |
| P3 | Warning | `DRAIN-003` | kube-system/coredns | Deployment kube-system/coredns has a nodeSelector/required nodeAffinity satisfied by only 1 node(s) in this cluster today (ip-192-168-52-167.ec2.internal) — if that node is drained, no other currently-known node can host a replacement pod |
| P3 | Warning | `EKS-NG-002` | ng-small | Managed node group "ng-small" desired size equals or is below minimum size. Rolling update may have limited disruption headroom. |
| P4 | Warning | `WH-005` | vpc-resource-validating-webhook | ValidatingWebhookConfiguration "vpc-resource-validating-webhook": webhook "vnode.vpc.k8s.aws" (index 1 in .webhooks) matches nodes — a fail-closed webhook here can block node status updates, namespace lifecycle, or PersistentVolume operations that upgrade/maintenance workflows depend on |
| P4 | Warning | `WORKLOAD-001` | preflight-case-study/already-broken-app-795cc7b4cd-4m4xn | Workload has unhealthy pods before upgrade: 1 pod in ImagePullBackOff. This workload was unhealthy before the upgrade, which can make post-upgrade validation ambiguous. |
| P4 | Info | `EKS-INSIGHT-003` | EKS add-on version compatibility | EKS upgrade insight "EKS add-on version compatibility" reports UNKNOWN for Kubernetes 1.32. Treat this as AWS-native context and verify with a fresh scan before upgrade. |
| P4 | Info | `EKS-NG-003` | ng-small | Managed node group "ng-small" uses a launch template/custom AMI. Validate AMI, bootstrap, kubelet, and launch template upgrade path manually. |
| P4 | Info | `EKS-NG-004` | ng-small | Managed node group "ng-small" reports Kubernetes version 1.31 while target is 1.32. Node kubelet skew is evaluated separately by NODE-001. |

