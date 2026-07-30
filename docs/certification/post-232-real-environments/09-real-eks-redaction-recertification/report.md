# Real EKS Redaction Re-Certification

Date: 2026-07-30.

This lane re-ran the redaction defects fixed by PR #235 against a fresh, disposable, non-production Amazon EKS cluster using the real `kubepreflight` binary built from merged `master` (`6d1254a`). It is certification-only: no rule semantics, RBAC scope, controller inventory, AKS/GKE, admission simulation, remediation, upgrade execution, or rollback execution work was performed.

## Environment

Disposable cluster shape:

- Cluster name: `kubepreflight-redact-cert-235`
- Region: `us-east-1`
- Kubernetes version: `1.32`
- Node group: one managed `t3.small` node
- NAT gateway: disabled
- Tags: `Project=KubePreflight`, `Purpose=RedactionCertificationPost235`, `AutoDelete=true`

The cluster was created only for this lane and deleted immediately after evidence capture. Setup and teardown used `eksctl`/`aws`; every `kubepreflight` invocation was read-only.

## Command Surfaces

Redaction-enabled and redaction-disabled pairs were captured for:

- `scan`
- `plan`
- `compare`
- `rollback plan`
- `rollback assess`

Outputs covered:

- terminal `stdout`
- terminal `stderr`
- JSON reports
- Markdown reports
- HTML reports where the command emits HTML (`scan`, `plan`, rollback)
- comparison JSON, Markdown, and gate JSON

## Identifier Coverage

The exact-value sweep used real identifiers collected from the disposable EKS cluster, node group, Kubernetes node inventory, and EC2 read-only describes. The corpus included real AWS account, ARN, VPC, subnet, security-group, instance, volume, IP, EC2 hostname, launch-template, ENI, and EKS API endpoint values.

Natural full-access EKS output emitted real ARNs, account IDs, and EC2 node hostnames in redaction-disabled mode. Because the clean full-access cluster did not naturally emit every cloud resource-ID family in report text, a temporary local deprecated-API manifest was added to a live EKS scan. Its resource name was composed from real VPC, subnet, security-group, instance, volume, and account identifiers from this same disposable cluster. That fixture was never applied to the cluster; it was scanned from disk alongside the live EKS evidence.

Results:

| Evidence pair | Redacted exact-value hits | Raw exact-value hits | Covered outputs |
|---|---:|---:|---|
| Natural EKS scan/plan/rollback/compare | 0 | 33 | stdout, JSON, Markdown, HTML, rollback, compare |
| Additive live-EKS scan with real-ID manifest fixture | 0 | raw IDs visible | stdout, `findings.json`, `report.md`, `report.html` |

The broad supported-pattern sweep over redaction-enabled outputs also returned no matches for AWS ARNs, account IDs, EKS API endpoint URLs, VPC IDs, subnet IDs, security-group IDs, instance IDs, volume IDs, ENI IDs, launch-template IDs, EC2 hostnames, or IPv4 addresses.

The EKS API endpoint was present in the read-only AWS describe corpus and absent from all retained redaction-enabled product outputs. Successful `kubepreflight` report surfaces did not naturally print the endpoint URL in raw mode.

## Semantic Invariants

Enabling redaction changed presentation only.

| Pair | Exit | Verdict | Score | Finding fingerprints / rule states |
|---|---:|---|---:|---|
| Natural EKS scan | 1 | `PASSED_WITH_WARNINGS` | 77 | identical |
| Natural EKS plan | 1 | `PASSED_WITH_WARNINGS` | 77 | identical |
| Live EKS scan + real-ID manifest fixture | 2 | `BLOCKED` | 62 | identical |
| `compare` | 0 | gate `pass` | n/a | comparison summary identical |
| `rollback plan` | 2 | recommendation `do_not_proceed` | n/a | readiness/recommendation identical |
| `rollback assess` | 2 | recommendation `do_not_proceed` | n/a | readiness/recommendation identical |

The real-ID manifest fixture pair preserved the same `API-001` fingerprint in redacted and raw mode: `8fb3a90dfd7120767eec98c90b7c4b3a5cd8873066606b6d8dc7c2c0f4dad3b9`.

## Read-Only Boundary

Product mutation checks:

- Kubernetes production-source mutation search found only local file writes (`os.Create`) in report/console output code.
- AWS collector/rollback mutation-method search found no mutating AWS SDK calls.
- The cluster remained `ACTIVE` and the node group remained `ACTIVE` with desired/min/max `1/1/1` after all product commands and before teardown.
- No `kubepreflight` command performed cluster setup, workload mutation, upgrade, rollback execution, drain, cordon, patch, delete, or AWS mutation.

Only certification setup/teardown mutated infrastructure, via `eksctl`/`aws` directly.

## Cleanup Verification

Cleanup command:

```bash
eksctl delete cluster --name kubepreflight-redact-cert-235 --region us-east-1 --wait
```

Independent post-delete checks:

- `aws eks describe-cluster` returned `ResourceNotFoundException`.
- `aws eks list-clusters --region us-east-1` returned an empty cluster list.
- CloudFormation stack query for the cluster name returned `[]`.
- The node EC2 instance was `terminated`.
- Cluster-owned EBS volumes query returned `[]`.
- Cluster VPC describe returned `InvalidVpcID.NotFound`.
- Cluster subnet describe returned `InvalidSubnetID.NotFound`.
- Cluster security-group describe returned `InvalidGroup.NotFound`.
- Cluster-tagged ENI query returned `[]`.

## Defect Status

This lane supports upgrading:

- `RED-TERMINAL-001` to **fixed, real-EKS verified**
- `RED-CLOUD-ID-002` to **fixed, real-EKS verified**

Fresh real-EKS redaction re-certification is complete for these two defects.
