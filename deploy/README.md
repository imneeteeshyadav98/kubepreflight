# Deploy artifacts

Copy-pasteable, read-only permissions for KubePreflight. Both files are derived directly from what the collectors in `internal/collectors/` actually call — see the comments in each for which check needs which grant.

## `clusterrole.yaml`

`kubectl apply -f deploy/clusterrole.yaml`, then point kubepreflight at a kubeconfig/token for the `kubepreflight` ServiceAccount it creates (adjust the namespace in both binding subjects if you're not running from `default`).

The `kube-system/coredns` ConfigMap read is intentionally a separate namespace-scoped `Role` with `resourceNames: ["coredns"]`, not folded into the cluster-wide `ClusterRole` — this is the RBAC-level enforcement of the "ConfigMap reads are allowlisted to known add-on configs, never blanket" principle, matching that the collector does a single `Get`, not a `List`.

No `secrets` verb appears anywhere in this file.

## `iam-policy.json`

Attach to whatever IAM principal (user, role, or IRSA-mapped ServiceAccount) runs `kubepreflight scan --provider eks`. All seven actions are read-only.

**On `Resource: "*"`:** several of these EKS actions (`DescribeCluster`, `ListInsights`, `DescribeInsight`, `ListAddons`, `DescribeAddon`) do support resource-level permissions scoped to a specific cluster ARN — but `DescribeAddonVersions` (queries the add-on catalog, not a specific cluster) and `ec2:DescribeSubnets` don't. Rather than ship a policy that mixes scoped and unscoped statements based on ARN syntax we haven't verified against a real account, this ships the safe, honest version: read-only, but unscoped by resource. If you want to tighten the cluster-specific actions to a single cluster ARN, check the current IAM action reference for the exact ARN format before doing so.
