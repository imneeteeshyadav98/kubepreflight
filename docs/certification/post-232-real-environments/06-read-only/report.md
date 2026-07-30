# Read-Only Verification

Date: 2026-07-29.

## 1. Source proof

See `source-proof.txt` in this directory for the full grep output. Summary:

- **Kubernetes mutation search**: `grep -rnE '\.(Create|Update|UpdateStatus|Patch|Delete|DeleteCollection|Evict|Bind|Apply|Scale)\(' --include="*.go" internal/ cmd/` (excluding `_test.go`) — every match is `os.Create` (local report-file writes in `internal/cli/compare.go` and `cmd/consoledevserver/main.go`). **Zero matches against any Kubernetes typed-client mutation method anywhere in production source.**
- **AWS SDK method search**: every AWS API call made anywhere in `internal/collectors/aws/` and `internal/rollback/eks/` is `Describe*` or `List*` — the complete set: `DescribeAddon`, `DescribeAddonVersions`, `DescribeCluster`, `DescribeClusterVersions`, `DescribeInsight`, `DescribeNodegroup`, `DescribeSecurityGroups`, `DescribeSubnets`, `DescribeUpdate`, `DescribeVpcs`, `ListAddons`, `ListInsights`, `ListNodegroups`, `ListUpdates`. `DescribeUpdate`/`ListUpdates` check status of any in-progress update; neither starts one. **No mutating AWS API call exists anywhere in production source.**

## 2. RBAC proof (Lane 1)

Every reduced-RBAC identity's `ClusterRole` granted only `["get", "list"]` verbs — never `create`, `update`, `patch`, `delete`, or `watch`. This makes mutation structurally impossible at the Kubernetes API-server level for those tokens, independent of product intent. Cross-checked against real cluster state: `kube-system` pod count and identity were unchanged (9 pods, same names/UIDs) across all 8 Lane 1 runs (full-access baseline + documented-baseline + 6 profiles).

## 3. IAM proof (Lane 2)

Every reduced-IAM policy in Lane 2 granted only `eks:Describe*`/`eks:List*` and `ec2:Describe*` actions — never any mutating EKS or EC2 action (`CreateCluster`, `UpdateClusterVersion`, `UpdateNodegroupVersion`, `StartUpdate`, `DeleteCluster`, etc.). The IAM policy documents themselves are retained (sanitized) as the direct evidence of this.

## 4. Runtime API evidence

CloudTrail lookup was not performed in this session (out of scope for the certification window given AWS CLI/CloudTrail query permissions were not separately verified, and the account's own restrictions — see Lane 2 — made additional AWS API exploration lower-value relative to the time budget). This is a disclosed limitation: runtime API-call-level proof rests on (a) the source-code proof above (no mutating call exists in the binary at all, so no CloudTrail record of one could exist regardless), (b) the structural RBAC/IAM proof (mutation was never even possible given the grants used), and (c) direct observation of cluster/AWS resource state before and after every scan (pod counts, node counts, CloudFormation stack states) showing no unexpected change. This is not request-level proof and is not claimed to be.

## 5. Certification setup mutations (explicitly not product behavior)

The following mutations happened during this certification, all via `kubectl`/`eksctl`/`aws` CLI directly — never through the `kubepreflight` binary:

- Kind cluster creation/deletion (Lane 1).
- ServiceAccounts, ClusterRoles, ClusterRoleBindings, Roles, RoleBindings created and deleted for RBAC fixtures (Lane 1) and the IAM-federated access-entry group binding (Lane 2).
- EKS cluster, managed node group, and their CloudFormation stacks created and deleted (Lane 2).
- One IAM user, its inline policies, access key, and EKS access entry created and deleted (Lane 2).

None of the above were invoked by `kubepreflight scan`/`compare`/`rollback` — they are test-environment setup/teardown, performed directly by the certification harness (this session), fully independent of the product's own code path. See "Test environment mutation vs. product behavior" in the top-level `README.md` for this distinction.

## Capability labels

- Kubernetes mutation absence: **real-binary verified** (source-level, exhaustive grep) + **real-cluster verified** (structural RBAC + observed state, both Kind and EKS).
- AWS mutation absence: **real-binary verified** (source-level) + **real-EKS verified** (structural IAM + observed CloudFormation/resource state).
- Runtime request-level (CloudTrail) proof: **not evaluated** this session — disclosed limitation, not claimed.
