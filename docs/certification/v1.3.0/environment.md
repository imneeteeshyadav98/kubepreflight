# Environment prerequisites, IAM policies, and sanitization plan

## AWS account / region / cluster

Placeholders only — a human fills these in at Stage 2 approval
(`approval-record.md`), never invented here:

- AWS account: `<PLACEHOLDER-SANDBOX-ACCOUNT>` — must be a
  non-production, disposable/sandbox account, mirroring
  `demo/eks-case-study/README.md`'s own prerequisite ("Use a
  sandbox/non-production AWS account").
- Region: `<PLACEHOLDER-REGION>` — `us-east-1` is the established
  convention in this repo (`demo/eks-case-study/cluster.yaml`,
  `demo/eks/`), but any region with EKS + the target instance type
  available is acceptable; nothing in this certification is
  region-sensitive.
- Cluster name: `<PLACEHOLDER-CLUSTER-NAME>` — recommend reusing
  `kubepreflight-case-study` or a clearly-scoped variant (e.g.
  `kubepreflight-v1.3.0-cert`) so it's unambiguous in `aws eks
  list-clusters` output and never collides with an unrelated cluster.
- Target Kubernetes version: same cluster, same target-version pair used
  for both full-access and reduced-IAM scans (recommend reusing the
  existing case study's already-proven `1.31` → `1.32` pair, or the
  latest EKS-supported pair at execution time — either is fine, nothing
  in this PR depends on a specific version).

## Tooling prerequisites

Same as `demo/eks-case-study/README.md`'s own prerequisites list:

- `aws` CLI, configured with credentials that can create/describe/delete
  an EKS cluster and its node group (for cluster setup/teardown only —
  see "IAM policies" below for what the **scanning** identities need,
  which is much narrower).
- [`eksctl`](https://eksctl.io/).
- `kubectl`.
- A local `kubepreflight` build on `PATH` (`go build -o kubepreflight
  ./cmd/kubepreflight`), or `KUBEPREFLIGHT_BIN` set to its path — same
  convention `scripts/case-study/*.sh` already use.
- `AWS_PROFILE`/`AWS_REGION` exported per mode (see below) — this
  certification, like the existing case study, never assumes or sets an
  AWS profile itself.
- An explicit `--context <name>` on every `kubepreflight scan --provider eks`
  command (see `commands.md`'s Phase 2/3 note) — never relies on whatever
  kubeconfig context happens to be current in the shell. `rollback assess`
  does not take this flag and does not need it: it only calls
  `internal/rollback/eks/collector.go`'s AWS SDK operations, never a
  Kubernetes API client (confirmed by reading `internal/cli/rollback.go`,
  which imports `internal/collectors/k8s` only for its
  `k8s.DefaultCollectorTimeout` flag-default constant, not for any client
  construction).
- `jq`, for the assertion scripts in `scripts/certification/`.

## Two AWS identities: full-access and reduced-IAM

Both modes scan the **same cluster**; only the local AWS credential
profile/role used to run `kubepreflight scan --provider eks` differs.
Both policies below are derived directly from the AWS SDK calls this
codebase actually makes — no action is included or excluded by guess:

- `internal/collectors/aws/collector.go` (`EKSClient`/`EC2Client`
  interfaces) — the calls a `scan --provider eks` makes.
- `internal/rollback/eks/collector.go` (`Client` interface) — the
  additional calls `rollback assess`/`rollback plan` make.

Every one of these is a `Describe*`/`List*` read call — nothing in
either policy grants any mutating EKS/EC2/IAM permission, and neither
policy is used for anything beyond running `kubepreflight` itself
(cluster create/upgrade/delete uses the separate, already-documented
`eksctl`/eksctl-managed IAM path from `demo/eks-case-study/README.md`,
not these policies).

### Full-access policy (`kubepreflight-cert-full-access`)

Grants every read action either collector calls, for the one cluster
under test (scope the `Resource` ARNs to that cluster/account/region at
Stage 2 — left as `*` here only because the concrete cluster ARN isn't
known until the cluster exists):

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "KubePreflightEKSReadFull",
      "Effect": "Allow",
      "Action": [
        "eks:DescribeCluster",
        "eks:ListInsights",
        "eks:DescribeInsight",
        "eks:ListAddons",
        "eks:DescribeAddon",
        "eks:DescribeAddonVersions",
        "eks:ListNodegroups",
        "eks:DescribeNodegroup",
        "eks:ListUpdates",
        "eks:DescribeUpdate",
        "eks:DescribeClusterVersions"
      ],
      "Resource": "*"
    },
    {
      "Sid": "KubePreflightEC2ReadFull",
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeSubnets",
        "ec2:DescribeSecurityGroups",
        "ec2:DescribeVpcs"
      ],
      "Resource": "*"
    }
  ]
}
```

`eks:DescribeCluster`/`eks:ListInsights`/`eks:DescribeInsight` are used
by both `scan --provider eks` (EKS-INSIGHT rules) and `rollback assess`
(rollback-readiness insights, a separate `Category` filter on the same
two API operations); `eks:ListUpdates`/`eks:DescribeUpdate`/
`eks:DescribeClusterVersions` are `rollback assess`-only.

This policy does **not** include `eks:AccessKubernetesApi` / cluster
`aws-auth`/access-entry RBAC mapping — that's a separate concern (how
the identity authenticates to the Kubernetes API for the `kubectl`-style
collector calls in `internal/collectors/k8s/`), not an EKS/EC2 IAM
action. Both full-access and reduced-IAM identities need the same
cluster-side read RBAC (`get`/`list`/`watch` on the resource kinds
`internal/collectors/k8s/collector.go` reads) — this plan does not
narrow that side, since "reduced IAM" in this PR is specifically about
the AWS-enrichment plane, not Kubernetes RBAC.

### Reduced-IAM policy (`kubepreflight-cert-reduced-iam`)

Deliberately omits the add-on, node group, and EC2-network read actions
— the AWS collector calls these but the CLI's own scan-time contract
(`internal/collectors/aws/collector.go`'s doc comment: "no credentials,
no IAM permissions... is a perfectly normal way to run this tool") says
a missing permission must degrade gracefully, never abort the scan:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "KubePreflightEKSReadReduced",
      "Effect": "Allow",
      "Action": [
        "eks:DescribeCluster",
        "eks:ListInsights",
        "eks:DescribeInsight"
      ],
      "Resource": "*"
    }
  ]
}
```

Explicitly **omitted** (no matching `Allow`, and no explicit `Deny`
needed — IAM defaults to deny): `eks:ListAddons`, `eks:DescribeAddon`,
`eks:DescribeAddonVersions`, `eks:ListNodegroups`,
`eks:DescribeNodegroup`, `eks:ListUpdates`, `eks:DescribeUpdate`,
`eks:DescribeClusterVersions`, `ec2:DescribeSubnets`,
`ec2:DescribeSecurityGroups`, `ec2:DescribeVpcs`.

Effect on the AWS collector, traced through the actual code
(`internal/collectors/aws/collector.go`'s `Collect`):

- `DescribeCluster` succeeds (kept) → `Snapshot.ClusterVersion`,
  `VpcID`, `EndpointAccess`, `PlatformVersion`, `Status`, `SupportType`,
  `ARN` are all still populated, and `report.EKSCluster` still renders.
- `ListInsights`/`DescribeInsight` succeed (kept) →
  `EKS-INSIGHT-001/002/003` still evaluate against real data.
- `collectAddons` (`ListAddons`) fails with `AccessDenied` →
  `Snapshot.Errors["list-addons"]` set, `Snapshot.Addons` stays empty →
  `ADDON-001` still runs (it is not one of the six rules with an
  explicit collector-error skip guard — see
  `internal/rules/execution.go`'s `ruleErrorsMapKeys`) but has nothing
  to evaluate, so it produces zero findings on an empty add-on list, not
  a `failed`/`insufficient_evidence` `RuleExecutionRecord`. This is a
  **known, load-bearing scope boundary to certify accurately, not
  paper over**: `ruleErrorsMapKeys` only tracks `WH-002`, `PDB-001`,
  `PDB-002`, `CRD-001`, `CRD-002`, `APISERVICE-001` — all six read
  `sc.K8s.Errors`, none read `sc.AWS.Errors`. No AWS-plane rule
  (`ADDON-001`, `ADDON-002`, `NODE-002`, `NET-002`, `EKS-NG-001..004`)
  can currently produce `insufficient_evidence` from an IAM-denied AWS
  call; they will show `Applicability: applicable, State: evaluated`
  even when their underlying AWS evidence was denied, not missing for
  some other benign reason.
- `collectNodegroups` (`ListNodegroups`) fails the same way →
  `EKS-NG-001..004` evaluate against an empty `Nodegroups` slice.
- `collectSubnets`/`collectNetworkPreflight` (`DescribeSubnets`,
  `DescribeSecurityGroups`, `DescribeVpcs`) fail the same way →
  `NODE-002`/`NET-002` evaluate against empty `Subnets`/
  `NetworkPreflightIssues`.
- Every one of the above denied calls lands in `Snapshot.Errors`, which
  `internal/cli/coverage.go`'s `buildScanCoverage` turns into
  `Coverage.AWS.Status = "partial"` with `Coverage.AWS.Errors` listing
  each denied operation (via `stableErrors`/`classifyCollectionIssue`) —
  **this is where the IAM restriction is actually visible** in the
  report: the top-level AWS plane coverage, not individual
  `RuleExecutionRecord.Reason` strings for the affected rules. See
  `expected-results.md`'s reduced-IAM section for the exact assertion
  this implies.

## Sanitization plan

### What `--redact-sensitive-identifiers` covers

`internal/redact/redact.go` defines exactly three patterns, applied to
every string field `internal/redact/report.go`/`Comparison`/
`RollbackAssessment`/`PlanReport` walk:

1. `arnPattern` — any `arn:aws:...` ARN (any service, any 12-digit
   account ID) → `[redacted-arn]`.
2. `hostnamePattern` — EC2-style node hostnames,
   `ip-10-0-1-100.ec2.internal` or
   `ip-10-0-1-100.us-east-1.compute.internal` → `[redacted-node-hostname]`.
3. `accountIDPattern` — a bare 12-digit number with word boundaries on
   both sides (catches an account ID in free text like "AccessDenied
   for account 000000000000", outside any ARN) → `[redacted-account-id]`.

Applied (per `internal/redact/report.go`) to: `ClusterContext`,
`Coverage.*.Errors`, `EKSCluster.ARN`, `APICompatibilitySummary`
resource-name lists, `EKSNodegroups[].AutoScalingGroups`/
`HealthIssues[].Message`/`.ResourceIDs`, `EKSUpgradeInsights[]`
description/recommendation/deprecation/addon-compatibility/
`AdditionalInfo` values, and every `Finding`'s `Message`, `Evidence`,
`Remediation`, `Resources[].Name/ProviderID/ProviderName`, and
`RemediationDetail` (diff, verify command, expected result, changes,
and every remediation action's command/steps). `Comparison` redaction
covers the same finding fields inside `New`/`Resolved`/`Unchanged`/
`NotReEvaluated`/`Changed`. `RollbackAssessment` redaction covers
`Checks[].Evidence` only — `Cluster.Name`/`Region` are deliberately left
alone (treated as non-sensitive, per that function's own doc comment).

### What it does **not** cover — confirmed from the same file, not assumed

- **VPC IDs** (`vpc-0123456789abcdef0`), **subnet IDs**
  (`subnet-0123456789abcdef0`), and **security group IDs**
  (`sg-0123456789abcdef0`) — none of the three regexes match these
  formats. `Snapshot.VpcID`, `SubnetRecord.ID`, and
  `NetworkPreflightIssue.ID` (and anything derived from them in a
  finding's evidence/message text) pass through `redact.Text`
  unchanged.
- **Private IP addresses** in any form other than the specific
  `ip-<octets>.ec2.internal`/`.compute.internal` hostname shape — a bare
  IP like `10.0.1.100` embedded in free text is not matched by
  `hostnamePattern` (which requires the `ip-...-...-...-....(ec2|
  compute).internal` suffix) or by any other pattern.
- **The cluster API server endpoint URL**
  (`https://<id>.gr7.<region>.eks.amazonaws.com`) — not an ARN, not the
  node-hostname shape, and does not contain a bare 12-digit number.
- **IAM user/role/session names when they don't appear inside an ARN**
  — e.g. a role name alone in prose ("assumed role
  kubepreflight-cert-reduced-iam") is untouched; only the ARN form is
  matched.
- **The `ClusterName`/`Region` fields on `EKSClusterInfo` and
  `RollbackAssessment` itself** — both are explicitly, deliberately
  left unredacted by design (see `RollbackAssessment`'s doc comment:
  "treating a cluster name and region as non-sensitive").
- **Tokens/credentials** — never appear in `findings.Report` at all
  (the AWS SDK's credential chain is process-local; nothing in this
  codebase writes a token/secret into any report field), so there is
  nothing for `redact` to redact here, but this must still be verified
  by the sanitization check below rather than assumed, since a future
  collector error message could in principle echo one.
- **Free-text `Reason`/error strings** from collector failures — these
  **are** passed through `redact.Text` wherever they land on a redacted
  field (`Coverage.*.Errors`, `RuleExecutionRecord` is **not** currently
  walked by `redact.Report` at all — `RuleExecutionRecord.Reason` is
  never redacted today). This is a real, confirmed gap: a future
  collector error surfaced through `RuleExecutionRecord.Reason` (e.g.
  `sanitizeRuleError`'s output) would reach committed evidence
  unredacted if it ever happened to contain an ARN or account ID. No
  rule in this build currently produces such a `Reason` string, but the
  sanitization check below scans `ruleExecutions[].reason` anyway as a
  defense-in-depth measure, not because a leak is expected.

### Conclusion: `--redact-sensitive-identifiers` alone is not sufficient

Every scan/compare/rollback-assess command in `commands.md` that
produces evidence destined for this directory passes
`--redact-sensitive-identifiers`, **and** every evidence file must also
pass `scripts/certification/check-evidence-sanitized.sh` before being
committed — that script greps for exactly the gaps identified above
(VPC/subnet/security-group IDs, private IPs, the EKS endpoint URL
pattern, and — as a redundant belt-and-suspenders check — ARNs and
12-digit account IDs, in case redaction was accidentally skipped on a
given file).

### `scripts/certification/check-evidence-sanitized.sh`

See that file directly. Summary: takes a directory path, recursively
greps every text file for:

- `arn:aws:` (ARN prefix — redundant with `--redact-sensitive-identifiers`
  but cheap insurance against a forgotten flag).
- A bare 12-digit number with word boundaries (same account-ID pattern
  as `internal/redact/redact.go`'s `accountIDPattern`, reimplemented
  independently in `grep -E`, not by importing/shelling into the Go
  package — this check must work even against a report a future code
  change accidentally stops redacting, so it deliberately does not
  trust the same code path it is meant to catch a regression in).
- `vpc-[0-9a-f]{8,17}`, `subnet-[0-9a-f]{8,17}`, `sg-[0-9a-f]{8,17}` —
  EC2 resource ID formats `redact` does not cover.
- RFC 1918 private IP octet patterns (`10.`, `172.16-31.`, `192.168.`)
  followed by three more octets.
- The EKS cluster endpoint hostname shape
  (`*.eks.amazonaws.com`) and `ip-*.ec2.internal`/`*.compute.internal`
  (redundant with `hostnamePattern`, same insurance rationale as the ARN
  check above).

Exits non-zero and prints every offending file:line if anything matches;
exits 0 and prints a summary line otherwise. Tested against synthetic
fixtures (never real data) — see `validation-summary.md` for the test
run and its output.
