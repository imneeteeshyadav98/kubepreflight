# Deploy artifacts

Copy-pasteable, read-only permissions for KubePreflight. Both files are derived directly from what the collectors in `internal/collectors/` actually call — see the comments in each for which check needs which grant.

## `clusterrole.yaml`

`kubectl apply -f deploy/clusterrole.yaml`, then point kubepreflight at a kubeconfig/token for the `kubepreflight` ServiceAccount it creates (adjust the namespace in both binding subjects if you're not running from `default`).

This is the recommended role for complete Kubernetes-plane evidence collection. It does not make AWS/EKS provider evidence available; use `iam-policy.json` as well for `scan --provider eks`.

The `kube-system/coredns` ConfigMap read is intentionally a separate namespace-scoped `Role` with `resourceNames: ["coredns"]`, not folded into the cluster-wide `ClusterRole` — this is the RBAC-level enforcement of the "ConfigMap reads are allowlisted to known add-on configs, never blanket" principle, matching that the collector does a single `Get`, not a `List`.

No `secrets` verb appears anywhere in this file.

## Kubernetes permission matrix

| Evidence | API group/resource | Verbs | Rule families |
|---|---|---|---|
| Nodes | `core/nodes` | `get`, `list` | `NODE-*`, drain and workload readiness |
| Pods | `core/pods` | `get`, `list` | `PDB-*`, `DRAIN-*`, `WORKLOAD-001` |
| Services | `core/services` | `get`, `list` | webhook and CRD backend checks when referenced |
| PersistentVolumes | `core/persistentvolumes` | `get`, `list` | `DRAIN-002`, `WH-005` sensitive-scope analysis |
| PersistentVolumeClaims | `core/persistentvolumeclaims` | `get`, `list` | `DRAIN-002` |
| PodDisruptionBudgets | `policy/poddisruptionbudgets` | `get`, `list` | `PDB-*`, drain overlap, deprecated API catalog |
| Webhook configurations | `admissionregistration.k8s.io/validatingwebhookconfigurations`, `mutatingwebhookconfigurations` | `get`, `list` | `WH-*` |
| EndpointSlices | `discovery.k8s.io/endpointslices` | `get`, `list` | webhook and CRD backend checks when referenced |
| Workload controllers | `apps/deployments`, `daemonsets`, `replicasets`, `statefulsets` | `get`, `list` | `DRAIN-*`, `WORKLOAD-001`, deprecated API catalog |
| CRDs | `apiextensions.k8s.io/customresourcedefinitions` | `get`, `list` | `CRD-*`, deprecated API catalog |
| APIServices | `apiregistration.k8s.io/apiservices` | `get`, `list` | `APISERVICE-001`, deprecated API catalog |
| CoreDNS ConfigMap | `core/configmaps` resource name `kube-system/coredns` | `get` | `COREDNS-001` |
| Deprecated API catalog resources | explicit resources in `policy`, `extensions`, `autoscaling`, `networking.k8s.io`, `certificates.k8s.io`, `coordination.k8s.io`, `rbac.authorization.k8s.io`, `batch`, `events.k8s.io`, `node.k8s.io`, `flowcontrol.apiserver.k8s.io`, `scheduling.k8s.io`, and `storage.k8s.io` | `get`, `list` | `API-001`, `API-002` |

The role deliberately omits `watch` because KubePreflight performs point-in-time list/get collection rather than running as an informer or controller. It also omits all mutating verbs and `secrets`.

## `iam-policy.json`

Attach to whatever IAM principal (user, role, or IRSA-mapped ServiceAccount) runs `kubepreflight scan --provider eks`. All actions are read-only.

**On `Resource: "*"`:** several of these EKS actions (`DescribeCluster`, `ListInsights`, `DescribeInsight`, `ListAddons`, `DescribeAddon`, `ListNodegroups`, `DescribeNodegroup`) do support resource-level permissions scoped to a specific cluster ARN — but `DescribeAddonVersions` (queries the add-on catalog, not a specific cluster), `ec2:DescribeSubnets`, `ec2:DescribeSecurityGroups`, and `ec2:DescribeVpcs` don't. Rather than ship a policy that mixes scoped and unscoped statements based on ARN syntax we haven't verified against a real account, this ships the safe, honest version: read-only, but unscoped by resource. If you want to tighten the cluster-specific actions to a single cluster ARN, check the current IAM action reference for the exact ARN format before doing so.

`ec2:DescribeSecurityGroups` and `ec2:DescribeVpcs` are for NET-002, which verifies that the cluster's referenced security groups and VPC still exist.

`eks:ListNodegroups` and `eks:DescribeNodegroup` are for EKS managed node group inventory/readiness only. AWS does not return self-managed node groups from `ListNodegroups`.

`eks:ListInsights` and `eks:DescribeInsight` are for read-only EKS Upgrade Insights inventory/readiness. KubePreflight does not request or call `eks:StartInsightsRefresh`.
