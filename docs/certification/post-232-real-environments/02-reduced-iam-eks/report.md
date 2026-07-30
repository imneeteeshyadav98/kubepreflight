# Lane 2 — Non-Production Amazon EKS Reduced-IAM Certification

Date: 2026-07-29
Source commit: `6197911`
Environment: Amazon EKS, Kubernetes `1.34` (standard support), cluster tagged `purpose=kubepreflight-certification, disposable=true`. Region `us-east-1`.
Binary: same real `kubepreflight` binary as Lane 1.

## Cost and infrastructure

- Control plane: `$0.10/cluster-hour` (Amazon EKS standard pricing).
- Node group: 1x `t3.micro`, Spot capacity, `gp3` 20GiB root volume, public subnet (no NAT Gateway, no load balancer, no ingress, no EKS Auto Mode, no paid observability add-ons — only the default `vpc-cni`/`kube-proxy`/`coredns` addons plus EKS-installed `metrics-server`).
- Total wall-clock cluster lifetime: control plane created `2026-07-29T09:56:22Z`, deleted starting `2026-07-29T17:47:26Z` — approximately 7.85 hours end-to-end, but the node group was only running for a fraction of that (see recovery note below); AWS credits were not checked/relied upon.
- **Approximate total cost: well under $1**, comfortably inside the $3.00 ceiling.
- EKS was not described as free anywhere in this certification.

## Real-world recovery incident (disclosed in full)

The initial `t3.medium` Spot node group creation **failed** with `AsgInstanceLaunchFailures — InvalidParameterCombination: instance type not eligible for Free Tier`. This AWS account enforces a **free-tier-only EC2 instance restriction** beyond normal IAM (confirmed later to also block several EKS API actions — see the IAM-B/C/D/E/F note below). The control plane itself had already succeeded and was billing at $0.10/hr with zero compute attached. Recovery: deleted the failed `ROLLBACK_COMPLETE` node group CloudFormation stack (had to disable eksctl's default termination protection first), then created a new node group using `t3.micro` (free-tier eligible), which succeeded cleanly. No orphaned AWS resources resulted from the failed attempt — CloudFormation's own rollback had already cleaned up the partial node group before I intervened.

A second incident: the background shell running `eksctl create cluster` was silently torn down by a session boundary between turns, with no completion record. I verified directly against the AWS API (`aws eks list-clusters`, `describe-cluster`, `cloudformation list-stacks`) before assuming anything about the cluster's state, rather than either abandoning it as lost or blindly retrying a duplicate create — this is what surfaced the free-tier node group failure, which had gone unnoticed while the session was down.

## Full-access baseline (`--provider eks`, real cluster)

`--target-version 1.34 --upgrade-context audit-only`: `verdict PASSED_WITH_WARNINGS`, score 85, exit 1. `coverage.kubernetes.status: complete`, `coverage.aws.status: complete`. All 31 rules applicable (no `not_applicable` — every EKS-specific rule activates with `--provider eks`). Real findings included a genuine AWS-managed admission webhook (`vpc-resource-validating-webhook`, `WH-005`) neither fabricated nor expected — this is EKS's own VPC resource controller webhook, present on every EKS cluster. `--redact-sensitive-identifiers` correctly scrubbed the account ID and cluster ARN from the retained findings.json (0 raw matches on independent grep). See `../03-full-real-eks/00-full-access-audit-only/`.

## IAM-B through IAM-F: a second real, confirmed environmental finding

**This AWS account blocks `eks:ListAddons`, `eks:ListNodegroups`, and `eks:ListInsights` account-wide, independent of identity-policy grants.** Proven directly: `aws iam simulate-principal-policy` reported all three actions **`allowed`** under the test identity's policy, yet the live API calls consistently returned `403 AccessDeniedException` for all three — across every profile (B, C, D, E, F) regardless of which of the three I actually granted in that profile's policy. This is almost certainly an account-level Service Control Policy or equivalent guardrail outside kubepreflight's or this certification's control (the same account also enforces the free-tier-only EC2 instance restriction noted above — this looks like a deliberately restricted/sandboxed account).

**Consequence for this lane's scope**: IAM-B, IAM-C, IAM-E, and IAM-F could not be cleanly isolated as single-variable tests in this account, because `ListAddons`/`ListNodegroups`/`ListInsights` are unconditionally unavailable regardless of what each profile's policy grants. **IAM-A and IAM-D remain clean, valid, single-variable tests** — IAM-A denies everything except `DescribeCluster` by design (unaffected by the account restriction, since I intended to deny those actions anyway), and IAM-D's defining variable (EC2 `Describe*` access) is *not* one of the three blocked actions, so its result is a genuine, isolated confirmation.

**Critically, this is not a kubepreflight defect.** In every single profile, the product's behavior was completely honest: every rule whose evidence was actually missing (whether by my policy design or the account's own restriction) correctly reported `insufficient_evidence`, `coverage.aws.status` correctly read `partial`, the result correctly read `INCOMPLETE`, and exit code was correctly `3` — in all six profiles, with zero exceptions and zero false passes.

## Definitive resolution of the Lane 1 "stale comment" finding

IAM-A (`eks:DescribeCluster` only, full K8s access via a dedicated EKS access-entry group bound to the Lane-1-equivalent complete `ClusterRole`) produced a **clean, single-variable result**: `coverage.kubernetes.status: complete`, `coverage.aws.status: partial`, and **exactly the 11 AWS-plane rules** (`NODE-002`, `NET-002`, `ADDON-001`, `ADDON-002`, `EKS-NG-001..004`, `EKS-INSIGHT-001..003`) showed `insufficient_evidence` — no more, no fewer. This directly confirms, with a clean isolated test, that Lane 1's Finding 1 (stale `assert-findings.sh` allowlist / stale `ruleErrorsMapKeys` comment) extends to the AWS plane as well: **every AWS-plane rule now supports per-rule `insufficient_evidence`**, contradicting the doc comment in `internal/report/evaluation_coverage.go` that claims these rules have "no such guard."

## Finding 3 — `--redact-sensitive-identifiers` has two distinct gaps

Discovered while sanitizing this lane's own evidence, then verified precisely across all 6 profiles:

**3a.** `internal/redact/redact.go` has exactly three patterns — ARN, EC2-internal-hostname, and bare 12-digit account ID. There is **no pattern for VPC IDs, Security Group IDs, subnet IDs, instance IDs, or volume IDs**. Real, raw VPC/SG identifiers (`vpc-...`, `sg-...`) appeared unredacted inside `coverage.aws.errors[]` raw AWS SDK error strings in IAM-A/D's `findings.json`, despite `--redact-sensitive-identifiers` being explicitly passed.

**3b. (more serious)** Even for patterns that *do* exist, **terminal (`stdout`) output is not redacted at all**, while `findings.json`/`report.md`/`report.html` from the exact same run *are* correctly redacted. Confirmed with a clean per-profile sweep: 0 raw account-ID matches in every profile's `findings.json`/`report.md` (all 6), vs. 2–7 raw matches in every profile's `stdout.txt` (all 6). This directly contradicts `scan --help`'s own description of `--redact-sensitive-identifiers`, which names "terminal" explicitly as a covered output.

Both are real, precisely isolated (not a broken pipeline — ARN/account-ID redaction demonstrably works correctly for 3 of 4 output formats). All leaked identifiers were manually scrubbed from retained evidence before being committed to disk here; see `../07-redaction/report.md` for the full writeup and per-profile leak counts.

Severity: **Medium-High**, driven mainly by 3b — an operator following the documented flag and piping `--terminal-output full` into a CI log (the flag's own named use case) would leak real ARNs/account IDs while believing they were protected. Not fixed here per certification scope. Tracked formally as **`RED-TERMINAL-001`** (3b, P1/High — the most important finding in this certification) and **`RED-CLOUD-ID-002`** (3a, P2/Medium) — see `../DEFECTS.md` for full acceptance criteria.

## Lane 2 results

| IAM profile | Allowed evidence | Missing evidence | Affected rules | Coverage | Result | Exit |
|---|---|---|---|---|---|---:|
| Full access | everything | none | none | complete/complete | PASSED_WITH_WARNINGS | 1 |
| A — DescribeCluster only | K8s: full (via access-entry group); AWS: DescribeCluster only | ListAddons, ListNodegroups, ListInsights, EC2 Describe* | exactly 11 AWS-plane rules (NODE-002, NET-002, ADDON-001/002, EKS-NG-001..004, EKS-INSIGHT-001..003) | K8s complete / AWS partial | INCOMPLETE | 3 |
| B — no add-on access (intended) | NG, Insights, EC2 (as policy) | Addons **+ NG + Insights (account-wide block, not policy)** | ADDON-001/002, EKS-NG-001..004, EKS-INSIGHT-001..003 | K8s complete / AWS partial | INCOMPLETE | 3 |
| C — no add-on access, EC2 variant | same pattern | same account-wide block | same 9 rules | K8s complete / AWS partial | INCOMPLETE | 3 |
| D — no EC2 network access | Addons, NG, Insights (as policy, but 2 of 3 blocked account-wide) | EC2 Describe* (by policy) **+ Addons/NG/Insights (account-wide)** | NODE-002, NET-002 (the clean, intended signal) **+** ADDON-001/002, EKS-NG-001..004, EKS-INSIGHT-001..003 (contaminated by the account block) | K8s complete / AWS partial | INCOMPLETE | 3 |
| E — no Insights access (intended) | Addons, NG, EC2 (as policy) | Insights **+ Addons + NG (account-wide)** | same 9-rule pattern | K8s complete / AWS partial | INCOMPLETE | 3 |
| F — documented complete policy (intended) | everything I could grant | Addons, NG, Insights still blocked account-wide | ADDON-001/002, EKS-INSIGHT-001..003 | K8s complete / AWS partial | INCOMPLETE | 3 |

**Note on Profile F's label:** "documented complete policy" describes the IAM policy document I attached, not the actual evidence the run obtained — this account's own restriction still blocked `ListAddons`/`ListNodegroups`/`ListInsights` even here, so Profile F did **not** achieve `coverage.aws.status: complete` and should not be read as a "full-IAM" baseline. The only run in this lane with genuinely complete AWS-plane evidence is the full-access baseline above (which used the account's actual admin/root path, not a constructed IAM policy).

## Lane 2 acceptance criteria

For every profile: exact allowed/denied actions recorded above; AWS caller identity captured and sanitized (never restated with raw account ID/ARN anywhere in this report); real binary used throughout; report artifacts captured for all 7 runs; dependent-only rules affected — confirmed via IAM-A and IAM-D's clean isolation, and via the fact that **zero K8s-plane rules were ever affected** in any IAM profile (K8s access was granted identically and fully in every profile); no missing permission ever produced a false pass or false blocker — confirmed across all 7 runs; exit code and coverage correct in all 7 runs; no mutating AWS API called (see `../06-read-only/report.md`). Temporary IAM user, policies, and access entry deleted and independently verified absent (`aws iam get-user` → `NoSuchEntity`).
