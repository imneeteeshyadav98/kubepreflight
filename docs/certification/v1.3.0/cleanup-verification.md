# Cleanup verification — v1.3.0 real-EKS certification

All live AWS and Kubernetes infrastructure created for this certification
has been deleted and independently re-verified absent, via fresh
read-only calls (never trusting a piped exit code alone — verified from
actual command/log output in every case). Full narrative detail and
timestamps are in `approval-record.md`; this file is the concise,
point-in-time verification checklist.

## 10-point independent verification — all passed

| # | Check | Method | Result |
|---|---|---|---|
| 1 | EKS cluster absent | `aws eks list-clusters --region ap-south-1` | empty — PASS |
| 2 | Managed node group absent | `aws eks list-nodegroups` | `ResourceNotFoundException` (cluster gone) — PASS |
| 3 | EC2 instance terminated | `aws ec2 describe-instances` filtered to this cluster's tag | none remain — PASS |
| 4 | EBS volume deleted | `aws ec2 describe-volumes` filtered to this cluster's tag | none remain — PASS |
| 5 | eksctl CloudFormation stacks deleted | `aws cloudformation list-stacks` | no `eksctl-kubepreflight-v130-cert-*` stacks remain — PASS |
| 6 | Temporary IAM role/policy absent | `aws iam get-role` | `NoSuchEntity` — PASS |
| 7 | Temporary kubeconfig context absent | `kubectl config get-contexts <name>` | not found — PASS |
| 8 | Original certification context removed correctly | `kubectl config get-contexts` | removed automatically by `eksctl delete cluster`'s own kubeconfig update, correctly, since it belonged to the deleted cluster — PASS |
| 9 | No other kubeconfig context touched | `kubectl config get-contexts` (full list) | all 9+ other pre-existing AWS-account contexts and `kind-kp-smoke` confirmed present and unmodified — PASS |
| 10 | Raw evidence preserved, gitignored, unstaged | `git status --short` + directory listing | all 5 raw evidence locations intact on disk, all confirmed gitignored, nothing staged — PASS |

## Cluster deletion — genuine success confirmed

`eksctl delete cluster --name kubepreflight-v130-cert --region ap-south-1 --wait`
completed with the tool's own real success marker in its log output
(`[✔] all cluster resources were deleted`), not inferred from a
`tee`-piped exit code alone.

## Approval-window note

Cleanup was completed after the original approval window
(`2026-07-27 20:52:18 IST`) had expired during a session interruption —
disclosed immediately upon discovery. Only the already-collected,
purely-local `compare` run and its sanitization scan occurred in the few
minutes between expiration and discovery; no new AWS or Kubernetes API
call occurred past the expiry boundary (confirmed: `compare` never
constructs an AWS or Kubernetes client). See `approval-record.md` for
the full disclosure.

## Result

**All live infrastructure for this certification is fully torn down.**
Remaining/completed work (Stage 4: sanitized evidence package,
assertions, checksums, this document) is entirely local-only.
