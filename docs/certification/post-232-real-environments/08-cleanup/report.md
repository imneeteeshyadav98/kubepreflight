# Cleanup Verification

Date: 2026-07-29.

## Local (Kind, Lane 1)

| Resource | Action | Independent verification |
|---|---|---|
| Kind cluster `kp-cert-rbac` | Deleted (`kind delete cluster`) | `kind get clusters` shows only the pre-existing, untouched `kp-smoke` cluster |
| 7× ServiceAccount, ClusterRole, ClusterRoleBinding (`kp-cert-a`..`kp-cert-f`, `kp-cert-documented`) | Deleted individually | `kubectl get clusterrole,clusterrolebinding,sa -A \| grep kp-cert` → no output before cluster deletion |
| 7× `kube-system` CoreDNS Role/RoleBinding | Deleted individually | Same sweep as above |
| Temporary kubeconfigs (`kubeconfig-<profile>.yaml`) | Deleted immediately after each scan | Not retained on disk at any point after use |

## AWS (Lane 2)

| Resource | Action | Independent verification |
|---|---|---|
| EKS cluster `kp-cert-0729-a0fb234a` | Deleted (`eksctl delete cluster --wait`) | `aws eks list-clusters --region us-east-1` → `{"clusters": []}` |
| Managed node group `cert-ng2` (and the earlier failed `cert-ng` attempt) | Deleted (both the corrected node group via `eksctl delete cluster`, and the failed attempt's stuck `ROLLBACK_COMPLETE` stack deleted manually mid-session after disabling termination protection) | `aws cloudformation list-stacks` — no `kp-cert` stack remains in any non-terminal-deleted state |
| CloudFormation stacks: `eksctl-kp-cert-0729-a0fb234a-cluster`, `eksctl-kp-cert-0729-a0fb234a-nodegroup-cert-ng2`, `eksctl-kp-cert-0729-a0fb234a-nodegroup-cert-ng` (failed attempt) | All deleted | Confirmed via `list-stacks` filter on `kp-cert` — empty |
| VPC + subnets + route tables + internet gateway (created by the cluster stack) | Deleted as part of the cluster CloudFormation stack teardown | `aws ec2 describe-vpcs --filters Name=tag:purpose,Values=kubepreflight-certification` → empty |
| Security groups (cluster + node group) | Deleted as part of stack teardown | Implicitly confirmed by VPC deletion (SGs cannot outlive their VPC) |
| EBS volume (node's `gp3` root volume) | Deleted with node/ASG termination | `aws ec2 describe-volumes --filters Name=tag:purpose,Values=kubepreflight-certification` → empty |
| Public IPv4 (node's public IP) | Released with node termination | `aws ec2 describe-addresses` filtered on the certification tag → empty |
| IAM user `kp-cert-iam-test` | Inline policies deleted, access key deleted, user deleted | `aws iam get-user --user-name kp-cert-iam-test` → `NoSuchEntity` |
| EKS access entry for the IAM user | Deleted (`aws eks delete-access-entry`) | Deleted before the cluster itself; cluster deletion additionally removes all access-entry state |
| IAM roles created by `eksctl` for the cluster/node group | Deleted as part of CloudFormation stack teardown (eksctl provisions cluster/node IAM roles inside the same stacks) | `aws iam list-roles` filtered on the cluster name → empty |
| Launch template (node group) | Deleted with the node group stack | Implicit in node group CloudFormation stack deletion |
| LoadBalancer/Ingress-created resources | None ever created | `kubectl get svc -A` (no non-ClusterIP services) and `kubectl get ingress -A` (no resources) checked immediately before cleanup began |
| Local kubeconfig entries (`kind-kp-cert-rbac`, `kp-cert-eks` alias) | Removed by the corresponding delete commands (`kind delete cluster` and `eksctl delete cluster` both clean their own kubeconfig context) | Not independently re-verified via a second `kubectl config get-contexts` pass — minor gap, no residual credential risk since neither points at a resource that still exists |
| Temporary files (`/tmp/kubepreflight-cert/**`, including `iam-test-keys.json`) | Never committed to the repository; `/tmp` is ephemeral and was in fact wiped mid-session by an unrelated session boundary, independently confirmed gone | `ls /tmp/kubepreflight-cert/` before rebuilding it mid-session returned "No such file or directory" |

## Cleanup acceptance criteria

- ✅ EKS cluster absent (verified).
- ✅ Node group absent (verified — no nodegroups list for the cluster, and the cluster itself is gone).
- ✅ Certification CloudFormation stacks deleted (verified, both the successful and the failed-attempt stacks).
- ✅ Certification VPC absent (verified).
- ✅ No orphaned volumes (verified).
- ✅ No orphaned Elastic IPs (verified).
- ✅ No temporary IAM role/policy/user (verified).
- No webhook/CRD fixtures — none were created this session (Cases E/F/G of Lane 3 rollback, which would have needed one, were not completed — see `../05-rollback/report.md`).
- ✅ Local Kind cluster deleted (verified).
- ✅ Temporary kubeconfigs deleted.
- ✅ No certification process left running (all scans/CLI invocations were synchronous foreground or explicitly-monitored background commands, none left dangling).
- Cleanup timestamps: control-plane creation `2026-07-29T09:56:22Z`; AWS cleanup start `2026-07-29T17:47:26Z`; `eksctl delete cluster` completion confirmed at `2026-07-29 23:29:27` (local/IST log timestamp) / independently re-verified via direct AWS API query immediately after.
- Estimated cost: see `../02-reduced-iam-eks/report.md` — well under $1, against the $3.00 ceiling.
